package player

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"netease-music-mygo/internal/netease"
)

// A song from the listener's folder opens from its file, MP3 or FLAC,
// without being streamed or cached.
func TestLocalFileOpens(t *testing.T) {
	for _, name := range []string{"seek.mp3", "seek.flac"} {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		p := &Player{}
		st, s, err := p.open(netease.Track{LocalPath: path, Duration: 1000}, "exhigh")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := strings.TrimPrefix(filepath.Ext(name), "."); st.Type != want {
			t.Errorf("%s opened as %q", name, st.Type)
		}
		buf := make([]float32, 4096)
		if n, _ := s.Read(buf); n == 0 {
			t.Errorf("%s decoded no samples", name)
		}
		s.Close()
	}
}
