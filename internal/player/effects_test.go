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
	s := newShaper(&tone{hz: hz, rate: 44100, frames: 44100}, &fx)
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
	s := newShaper(&tone{hz: 60, rate: 44100, frames: 44100}, &fx)
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
	s := newShaper(&scaled{quiet, 0.2}, &fx)
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
