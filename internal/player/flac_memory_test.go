package player

import (
	"io"
	"os"
	"testing"
)

// Reading a frame used to discard its buffer's capacity and allocate again.
func TestFLACReusesOutputBuffer(t *testing.T) {
	b, err := os.ReadFile("testdata/seek.flac")
	if err != nil {
		t.Fatal(err)
	}
	s, err := openFLACStream(downloaded(b))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, err := s.parse(s.r)
	if err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		s.decode(f)
		s.pend = s.pend[len(s.pend):]
	})
	if allocs != 0 {
		t.Fatalf("decoded output allocated %.0f times per frame", allocs)
	}
	t.Logf("output buffer allocations per warmed frame: %.0f", allocs)
}

// Partial reads must keep every sample when the next frame reuses storage.
func TestFLACSmallReads(t *testing.T) {
	b, err := os.ReadFile("testdata/seek.flac")
	if err != nil {
		t.Fatal(err)
	}
	want := decodeAll(t, b)
	s, err := openFLACStream(downloaded(b))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	buf := make([]float32, 258)
	at := 0
	for {
		n, err := s.Read(buf)
		if at+n > len(want) {
			t.Fatal("decoded more samples than the file contains")
		}
		if d := maxDiff(buf[:n], want[at:]); d != 0 {
			t.Fatalf("samples at %d differ by %g", at, d)
		}
		at += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if at != len(want) {
		t.Fatalf("decoded %d samples, want %d", at, len(want))
	}
}
