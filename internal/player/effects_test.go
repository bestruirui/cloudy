package player

import (
	"math"
	"sync/atomic"
	"testing"
)

// energy is the mean square of the left channel of a tone of hz through
// the effects e, once the filters have settled.
func energy(t *testing.T, hz float64, e *Effects) float64 {
	t.Helper()
	var fx atomic.Pointer[Effects]
	if e != nil {
		fx.Store(e)
	}
	s := newShaper(&tone{hz: hz, rate: 44100, frames: 44100}, &fx, nil)
	buf := make([]float32, 2*44100)
	n, _ := s.Read(buf)
	sum := 0.0
	for i := 2 * 4410; i < n; i += 2 {
		sum += float64(buf[i]) * float64(buf[i])
	}
	return sum / float64((n-2*4410)/2)
}

// Off, or on with nothing changed, the sound passes as it was; a band
// lifted makes its tone louder and leaves one far from it as it was, and
// the bass lifts the low tones only. Nothing goes past full scale.
func TestEffects(t *testing.T) {
	plain := energy(t, 1000, nil)
	if got := energy(t, 1000, &Effects{On: true}); math.Abs(got-plain) > plain*0.01 {
		t.Errorf("flat effects changed the sound: %v, was %v", got, plain)
	}
	var lift Effects
	lift.On, lift.Bands[5] = true, 9 // 1 kHz
	// 9 dB is about 8 times the energy, against a tone far from the band.
	if ratio := energy(t, 1000, &lift) / energy(t, 31, &lift); ratio < 6 || ratio > 10 {
		t.Errorf("1 kHz lifted 9 dB stands %.1f× over 31 Hz", ratio)
	}
	// The whole comes down to leave the lift room: far from the band, a
	// tone is as loud as the gain before it makes it, not louder.
	if got, far := energy(t, 31, &lift), energy(t, 31, nil); got > far*1.05 {
		t.Errorf("31 Hz, far from the band, went %v → %v", far, got)
	}
	bass := Effects{On: true, Bass: 1}
	low, high := energy(t, 50, &bass)/energy(t, 50, nil), energy(t, 5000, &bass)/energy(t, 5000, nil)
	if low < high*3 {
		t.Errorf("the bass lifted 50 Hz %.2f×, 5 kHz %.2f×", low, high)
	}
	loud := Effects{On: true, Bass: 1}
	for i := range loud.Bands {
		loud.Bands[i] = EffectRange
	}
	var fx atomic.Pointer[Effects]
	fx.Store(&loud)
	s := newShaper(&tone{hz: 60, rate: 44100, frames: 44100}, &fx, nil)
	buf := make([]float32, 2*44100)
	n, _ := s.Read(buf)
	for _, v := range buf[:n] {
		if v > 1 || v < -1 {
			t.Fatalf("a sample at %v", v)
		}
	}
}

// The voice, in the middle, comes forward against the sides; evened out,
// a quiet passage comes up and a loud one stays under full scale.
func TestVocalAndLoudness(t *testing.T) {
	vocal := Effects{On: true, Vocal: 1}
	if r := energy(t, 2800, &vocal) / energy(t, 2800, nil); r < 1.5 {
		t.Errorf("a voice at 2.8 kHz came up %.2f×", r)
	}
	quiet := &tone{hz: 440, rate: 44100, frames: 44100}
	loud := Effects{On: true, Loudness: 1}
	var fx atomic.Pointer[Effects]
	fx.Store(&loud)
	// A tone at a tenth of full scale, about -26 dBFS RMS.
	s := newShaper(&scaled{quiet, 0.2}, &fx, nil)
	buf := make([]float32, 2*44100)
	n, _ := s.Read(buf)
	peak := float32(0)
	for _, v := range buf[n/2 : n] {
		peak = max(peak, v, -v)
	}
	if peak < 0.15 || peak > 1 {
		t.Errorf("a quiet tone evened out peaks at %v, was 0.1", peak)
	}
}

// scaled is a decoder at k of its loudness.
type scaled struct {
	*tone
	k float32
}

func (s *scaled) Read(dst []float32) (int, error) {
	n, err := s.tone.Read(dst)
	for i := range dst[:n] {
		dst[i] *= s.k
	}
	return n, err
}

// The treble lifts the highs and leaves the lows; turned down, the bass
// takes the lows down; a room rings on after the sound stops; nothing goes
// past full scale with all of it at once.
func TestTrebleBassRoom(t *testing.T) {
	up := Effects{On: true, Treble: 1}
	if hi, lo := energy(t, 10000, &up)/energy(t, 10000, nil), energy(t, 100, &up)/energy(t, 100, nil); hi < lo*3 {
		t.Errorf("treble lifted 10 kHz %.2f×, 100 Hz %.2f×", hi, lo)
	}
	down := Effects{On: true, Bass: -1}
	if r := energy(t, 50, &down) / energy(t, 50, nil); r > 0.4 {
		t.Errorf("bass turned down left 50 Hz at %.2f×", r)
	}
	for _, room := range Rooms[1:] {
		e := Effects{On: true, Room: room.ID, Depth: 1, Width: 1, Bass: 1, Treble: 1}
		var fx atomic.Pointer[Effects]
		fx.Store(&e)
		// Half a second of tone, then silence.
		s := newShaper(&tone{hz: 440, rate: 44100, frames: 22050}, &fx, nil)
		buf := make([]float32, 2*22050)
		s.Read(buf)
		for _, v := range buf {
			if v > 1 || v < -1 {
				t.Fatalf("%s: a sample at %v", room.Label, v)
			}
		}
		tail := make([]float32, 2*4410)
		silent := &tone{hz: 0, rate: 44100, frames: 4410}
		s.Decoder = silent
		s.Read(tail)
		ring := float32(0)
		for _, v := range tail {
			ring = max(ring, v, -v)
		}
		if ring < 0.001 {
			t.Errorf("%s: nothing rang after the sound stopped", room.Label)
		}
	}
}

// earEnergy is each ear's energy over frames [from, to) of out.
func earEnergy(out []float32, from, to int) (l, r float64) {
	for i := from; i < to; i++ {
		l += float64(out[2*i]) * float64(out[2*i])
		r += float64(out[2*i+1]) * float64(out[2*i+1])
	}
	return l, r
}

// shaped runs a decoder through e, for frames.
func shaped(d interface {
	SampleRate() int
	Channels() int
	Read([]float32) (int, error)
	SetPosition(int64) error
	Len() int64
}, e Effects, frames int) []float32 {
	var fx atomic.Pointer[Effects]
	fx.Store(&e)
	s := newShaper(d, &fx, nil)
	out := make([]float32, 2*frames)
	for at := 0; at < len(out); {
		n, err := s.Read(out[at:min(len(out), at+8192)])
		at += n
		if err != nil || n == 0 {
			break
		}
	}
	return out
}

// 8D takes the sound round the head: a quarter of the way round, it is at
// the right; three quarters, at the left. 360° sets each side before the
// listener, so that a sound on the left alone reaches the right ear too.
func TestSpatial(t *testing.T) {
	const rate = 44100
	period := orbitPeriod(1) // 4 s
	frames := int(period * rate)
	out := shaped(&tone{hz: 440, rate: rate, frames: int64(frames)}, Effects{On: true, Spatial: Spatial8D, Orbit: 1}, frames)
	q := frames / 4
	l1, r1 := earEnergy(out, q-q/4, q+q/4)
	l3, r3 := earEnergy(out, 3*q-q/4, 3*q+q/4)
	if r1 < 2*l1 || l3 < 2*r3 {
		t.Errorf("8D: at a quarter L %.0f R %.0f; at three quarters L %.0f R %.0f", l1, r1, l3, r3)
	}

	left := &leftOnly{&tone{hz: 440, rate: rate, frames: rate}}
	out = shaped(left, Effects{On: true, Spatial: Spatial360}, rate)
	l, r := earEnergy(out, rate/4, rate)
	if r < l*0.1 || r > l {
		t.Errorf("360°: a sound on the left reached the ears L %.0f R %.0f", l, r)
	}
	for _, v := range out {
		if v > 1 || v < -1 {
			t.Fatalf("360°: a sample at %v", v)
		}
	}
}

// leftOnly is a decoder with its right channel silent.
type leftOnly struct{ *tone }

func (d *leftOnly) Read(dst []float32) (int, error) {
	n, err := d.tone.Read(dst)
	for i := 1; i < n; i += 2 {
		dst[i] = 0
	}
	return n, err
}

// Effects chosen for a song glide in: halfway through the glide, a band
// lifted is lifted about halfway, not at once nor not at all.
func TestGlide(t *testing.T) {
	const rate = 44100
	var fx atomic.Pointer[Effects]
	var glide atomic.Bool
	fx.Store(&Effects{On: true})
	s := newShaper(&tone{hz: 1000, rate: rate, frames: 10 * rate}, &fx, &glide)
	buf := make([]float32, 2*rate/2)
	s.Read(buf) // settled, flat
	flatE, _ := earEnergy(buf, rate/4-2000, rate/4+2000)
	var lift Effects
	lift.On, lift.Bands[5], lift.Bands[0] = true, 12, 12
	glide.Store(true)
	fx.Store(&lift)
	// The glide's first second: its middle.
	s.Read(buf)
	s.Read(buf)
	mid, _ := earEnergy(buf, rate/4-2000, rate/4+2000)
	for range 6 {
		s.Read(buf)
	}
	end, _ := earEnergy(buf, rate/4-2000, rate/4+2000)
	if !(mid > flatE*1.05 && mid < end*0.95) {
		t.Errorf("energy flat %.0f, mid-glide %.0f, after %.0f", flatE, mid, end)
	}
}
