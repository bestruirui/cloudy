package player

import (
	"io"
	"math"
	"testing"
)

// tone is a decoder of a sine wave of hz, frames long, in stereo.
type tone struct {
	hz, rate float64
	frames   int64
	at       int64
}

func (t *tone) SampleRate() int { return int(t.rate) }
func (t *tone) Channels() int   { return 2 }
func (t *tone) Len() int64      { return t.frames }
func (t *tone) SetPosition(f int64) error {
	t.at = f
	return nil
}
func (t *tone) Read(dst []float32) (int, error) {
	n := 0
	for ; n+1 < len(dst) && t.at < t.frames; n += 2 {
		v := float32(0.5 * math.Sin(2*math.Pi*t.hz*float64(t.at)/t.rate))
		dst[n], dst[n+1] = v, v
		t.at++
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

// An episode at 1.5 times lasts two thirds as long, and its voice keeps its
// pitch: the wave crosses zero as often a second as it did.
func TestStretchKeepsPitch(t *testing.T) {
	src := &tone{hz: 220, rate: 44100, frames: 44100 * 3}
	s := newStretch(src, 1.5)
	s.SetPosition(0)
	var out []float32
	buf := make([]float32, 4096)
	for {
		n, err := s.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	frames := len(out) / 2
	if want := 44100 * 2; math.Abs(float64(frames-want)) > 44100*0.05 {
		t.Errorf("%d frames, want about %d", frames, want)
	}
	// Away from the ends, which fade.
	crossings := 0
	mid := out[2*4410 : 2*(frames-4410)]
	for i := 2; i < len(mid); i += 2 {
		if (mid[i-2] < 0) != (mid[i] < 0) {
			crossings++
		}
	}
	secs := float64(len(mid)/2) / 44100
	if hz := float64(crossings) / 2 / secs; math.Abs(hz-220) > 6 {
		t.Errorf("plays at %.1f Hz, want 220", hz)
	}
	if s.Len() != int64(float64(src.frames)/1.5) {
		t.Errorf("Len %d, want %d", s.Len(), int64(float64(src.frames)/1.5))
	}
}

// At its own speed it passes the stream through untouched.
func TestStretchAtOneIsTheStream(t *testing.T) {
	src := &tone{hz: 440, rate: 44100, frames: 1000}
	s := newStretch(src, 1)
	a := make([]float32, 2000)
	n, _ := s.Read(a)
	ref := &tone{hz: 440, rate: 44100, frames: 1000}
	b := make([]float32, 2000)
	ref.Read(b)
	for i := range n {
		if a[i] != b[i] {
			t.Fatalf("sample %d differs", i)
		}
	}
}
