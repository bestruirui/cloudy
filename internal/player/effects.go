package player

import (
	"io"
	"math"
	"sync/atomic"

	"github.com/egoist/mygo/audio"
)

// Effects shape the sound of what plays: a ten-band equalizer, the bass
// lifted, and the stereo widened. They apply to every track, from the next
// moment of sound the player decodes.
type Effects struct {
	On    bool
	Bands [len(EffectBands)]float64 // gain in dB, of each of EffectBands
	Bass  float64                   // 0 to 1: up to 9 dB under 100 Hz
	Width float64                   // 0 to 1: the sides up to twice as wide
}

// EffectBands are the middles of the equalizer's bands, in Hz, an octave
// apart.
var EffectBands = [10]float64{31, 62, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// EffectRange is how far a band goes up or down, in dB.
const EffectRange = 12

// SetEffects has what plays shaped by e, this track and the next. Heard at
// once, rather than after the second the player decodes ahead, when flush
// is set: what was decoded is decoded again.
func (p *Player) SetEffects(e Effects, flush bool) {
	p.fx.Store(&e)
	if flush && p.out != nil && p.out.Playing() && !p.seeking {
		p.Seek(p.Position())
	}
}

// biquad is a second-order filter, as the Audio EQ Cookbook gives them,
// normalized: b0, b1, b2 over a1, a2.
type biquad struct{ b0, b1, b2, a1, a2 float64 }

// biquadState is a filter's memory for one channel.
type biquadState struct{ x1, x2, y1, y2 float64 }

func (f *biquad) run(s *biquadState, x float64) float64 {
	y := f.b0*x + f.b1*s.x1 + f.b2*s.x2 - f.a1*s.y1 - f.a2*s.y2
	s.x2, s.x1 = s.x1, x
	s.y2, s.y1 = s.y1, y
	return y
}

// peaking lifts or cuts gain dB about f0, an octave wide.
func peaking(rate, f0, gain float64) biquad {
	A := math.Pow(10, gain/40)
	w := 2 * math.Pi * f0 / rate
	alpha := math.Sin(w) / (2 * 1.41)
	cos := math.Cos(w)
	a0 := 1 + alpha/A
	return biquad{
		b0: (1 + alpha*A) / a0, b1: -2 * cos / a0, b2: (1 - alpha*A) / a0,
		a1: -2 * cos / a0, a2: (1 - alpha/A) / a0,
	}
}

// lowShelf lifts what is under f0 by gain dB.
func lowShelf(rate, f0, gain float64) biquad {
	A := math.Pow(10, gain/40)
	w := 2 * math.Pi * f0 / rate
	cos, sin := math.Cos(w), math.Sin(w)
	alpha := sin / 2 * math.Sqrt2 // a shelf slope of 1
	sq := 2 * math.Sqrt(A) * alpha
	a0 := (A + 1) + (A-1)*cos + sq
	return biquad{
		b0: A * ((A + 1) - (A-1)*cos + sq) / a0,
		b1: 2 * A * ((A - 1) - (A+1)*cos) / a0,
		b2: A * ((A + 1) - (A-1)*cos - sq) / a0,
		a1: -2 * ((A - 1) + (A+1)*cos) / a0,
		a2: ((A + 1) + (A-1)*cos - sq) / a0,
	}
}

// bassGain is the most the bass lifts, in dB.
const bassGain = 9

// shaper applies the effects to a decoder's sound as it is read.
type shaper struct {
	audio.Decoder
	fx      *atomic.Pointer[Effects]
	applied Effects
	filters []biquad
	state   [][2]biquadState
	pre     float64 // the gain before, that leaves room for what is lifted
}

func newShaper(d audio.Decoder, fx *atomic.Pointer[Effects]) *shaper {
	return &shaper{Decoder: d, fx: fx}
}

// tune makes the filters of e.
func (s *shaper) tune(e Effects) {
	s.applied = e
	rate := float64(s.SampleRate())
	s.filters = s.filters[:0]
	most := 0.0
	for i, f := range EffectBands {
		if g := e.Bands[i]; g != 0 && f < rate/2*0.9 {
			s.filters = append(s.filters, peaking(rate, f, g))
			most = max(most, g)
		}
	}
	if e.Bass > 0 {
		g := e.Bass * bassGain
		s.filters = append(s.filters, lowShelf(rate, 100, g))
		most = max(most, g+max(e.Bands[0], e.Bands[1], 0))
	}
	// What is lifted could clip: the whole comes down as far as the most
	// lifted, a little less, for music seldom fills every band at once.
	s.pre = math.Pow(10, -most*0.8/20)
	s.state = make([][2]biquadState, len(s.filters))
}

func (s *shaper) Read(dst []float32) (int, error) {
	n, err := s.Decoder.Read(dst)
	e := s.fx.Load()
	if e == nil || !e.On {
		return n, err
	}
	if *e != s.applied || s.state == nil {
		s.tune(*e)
	}
	side := 1 + e.Width
	for i := 0; i+1 < n; i += 2 {
		l, r := float64(dst[i])*s.pre, float64(dst[i+1])*s.pre
		for k := range s.filters {
			f := &s.filters[k]
			l = f.run(&s.state[k][0], l)
			r = f.run(&s.state[k][1], r)
		}
		if side != 1 {
			mid, sd := (l+r)/2, (l-r)/2*side
			l, r = mid+sd, mid-sd
		}
		dst[i], dst[i+1] = float32(soften(l)), float32(soften(r))
	}
	return n, err
}

// soften keeps a sample in [-1, 1], rounding off what goes past 0.9
// rather than cutting it, which would crackle.
func soften(x float64) float64 {
	const knee = 0.9
	switch {
	case x > knee:
		return knee + (1-knee)*math.Tanh((x-knee)/(1-knee))
	case x < -knee:
		return -knee - (1-knee)*math.Tanh((-x-knee)/(1-knee))
	}
	return x
}

func (s *shaper) SetPosition(frame int64) error {
	// The filters' memory is of the sound before: none of it carries over.
	for i := range s.state {
		s.state[i] = [2]biquadState{}
	}
	return s.Decoder.SetPosition(frame)
}

func (s *shaper) Close() error {
	if c, ok := s.Decoder.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
