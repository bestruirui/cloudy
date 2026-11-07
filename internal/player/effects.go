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
	Bass  float64                   // -1 to 1: up to 9 dB under 100 Hz, down or up
	Width float64                   // 0 to 1: the sides up to twice as wide
	// Treble is the highs over 6 kHz, -1 to 1, up to 9 dB down or up;
	// Depth the sides given the depth of the other channel a moment late,
	// 0 to 1; Room the room the sound plays in, of Rooms.
	Treble float64
	Depth  float64
	Room   string
	// Vocal lifts what sits in the middle, the voice, where it is heard
	// clearly, 0 to 1; Loudness evens out what is loud and what is quiet,
	// 0 to 1, as a compressor does.
	Vocal    float64
	Loudness float64
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

// highShelf lifts what is over f0 by gain dB.
func highShelf(rate, f0, gain float64) biquad {
	A := math.Pow(10, gain/40)
	w := 2 * math.Pi * f0 / rate
	cos, sin := math.Cos(w), math.Sin(w)
	alpha := sin / 2 * math.Sqrt2
	sq := 2 * math.Sqrt(A) * alpha
	a0 := (A + 1) - (A-1)*cos + sq
	return biquad{
		b0: A * ((A + 1) + (A-1)*cos + sq) / a0,
		b1: -2 * A * ((A - 1) + (A+1)*cos) / a0,
		b2: A * ((A + 1) + (A-1)*cos - sq) / a0,
		a1: 2 * ((A - 1) - (A+1)*cos) / a0,
		a2: ((A + 1) - (A-1)*cos - sq) / a0,
	}
}

// bassGain and trebleGain are the most the bass and the highs go up or
// down, in dB.
const (
	bassGain   = 9
	trebleGain = 9
)

// depthDelay is how late the other channel comes back into each, for the
// depth: within the 1 to 30 ms the ear takes for a room's.
const depthDelay = 0.012

// shaper applies the effects to a decoder's sound as it is read.
type shaper struct {
	audio.Decoder
	fx              *atomic.Pointer[Effects]
	applied         Effects
	filters         []biquad
	state           [][2]biquadState
	pre             float64 // the gain before, that leaves room for what is lifted
	vocal           biquad  // the voice lifted, in the middle only
	voice           biquadState
	env             float64 // how loud the sound is now, for the compressor
	attack, release float64
	room            *reverb
	delay           [2][]float64 // each channel as it was, for the depth
	delayAt         int
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
	if e.Bass != 0 {
		g := e.Bass * bassGain
		s.filters = append(s.filters, lowShelf(rate, 100, g))
		most = max(most, g+max(e.Bands[0], e.Bands[1], 0))
	}
	if e.Treble != 0 {
		g := e.Treble * trebleGain
		s.filters = append(s.filters, highShelf(rate, 6000, g))
		most = max(most, g+max(e.Bands[8], e.Bands[9], 0))
	}
	s.room = nil
	if r := roomOf(e.Room); r != nil {
		s.room = newReverb(r, rate)
	}
	if n := int(depthDelay * rate); e.Depth > 0 && len(s.delay[0]) != n {
		s.delay = [2][]float64{make([]float64, n), make([]float64, n)}
		s.delayAt = 0
	}
	if e.Vocal > 0 {
		// Presence, where a voice is understood: in the middle of the
		// stereo, where a voice is mixed, not the instruments at the sides.
		s.vocal = peaking(rate, 2800, e.Vocal*vocalGain)
		most = max(most, e.Vocal*vocalGain*0.5)
	}
	// What is lifted could clip: the whole comes down as far as the most
	// lifted, a little less, for music seldom fills every band at once.
	s.pre = math.Pow(10, -most*0.8/20)
	s.state = make([][2]biquadState, len(s.filters))
	// The compressor follows the sound within 5 ms as it gets louder, and
	// lets go over a quarter of a second as it gets quieter.
	s.attack = 1 - math.Exp(-1/(0.005*rate))
	s.release = 1 - math.Exp(-1/(0.25*rate))
}

// vocalGain is the most the voice is lifted, in dB.
const vocalGain = 6

// The compressor of Loudness: above compressThreshold the sound grows by
// a quarter of what it would at the most, made up by up to
// compressMakeup dB, so that quiet passages come up and loud ones down.
const (
	compressThreshold = -20.0 // dBFS
	compressMakeup    = 8.0   // dB
)

func (s *shaper) Read(dst []float32) (int, error) {
	n, err := s.Decoder.Read(dst)
	e := s.fx.Load()
	if e == nil || !e.On {
		return n, err
	}
	if *e != s.applied || s.state == nil {
		s.tune(*e)
	}
	side := (1 + e.Width) * (1 - 0.3*e.Vocal)
	ratio := 1 + 3*e.Loudness
	makeup := math.Pow(10, e.Loudness*compressMakeup/20)
	for i := 0; i+1 < n; i += 2 {
		l, r := float64(dst[i])*s.pre, float64(dst[i+1])*s.pre
		for k := range s.filters {
			f := &s.filters[k]
			l = f.run(&s.state[k][0], l)
			r = f.run(&s.state[k][1], r)
		}
		if side != 1 || e.Vocal > 0 {
			mid, sd := (l+r)/2, (l-r)/2*side
			if e.Vocal > 0 {
				mid = s.vocal.run(&s.voice, mid)
			}
			l, r = mid+sd, mid-sd
		}
		if e.Depth > 0 && len(s.delay[0]) > 0 {
			// Each channel takes the other's, a moment late and turned
			// over: what is at the sides seems to come from further off.
			k := 0.45 * e.Depth
			dl, dr := s.delay[0][s.delayAt], s.delay[1][s.delayAt]
			s.delay[0][s.delayAt], s.delay[1][s.delayAt] = l, r
			if s.delayAt++; s.delayAt == len(s.delay[0]) {
				s.delayAt = 0
			}
			l, r = l-k*dr, r-k*dl
		}
		if s.room != nil {
			l, r = s.room.run(l, r)
		}
		if e.Loudness > 0 {
			level := max(math.Abs(l), math.Abs(r))
			if level > s.env {
				s.env += s.attack * (level - s.env)
			} else {
				s.env += s.release * (level - s.env)
			}
			g := makeup
			if db := 20 * math.Log10(s.env+1e-9); db > compressThreshold {
				g *= math.Pow(10, (db-compressThreshold)*(1/ratio-1)/20)
			}
			l, r = l*g, r*g
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
	s.voice, s.env = biquadState{}, 0
	for _, d := range s.delay {
		clear(d)
	}
	if s.room != nil {
		// A room still ringing with what played before would ring on.
		s.room = newReverb(roomOf(s.applied.Room), float64(s.SampleRate()))
	}
	return s.Decoder.SetPosition(frame)
}

func (s *shaper) Close() error {
	if c, ok := s.Decoder.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
