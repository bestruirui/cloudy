package app

import (
	"runtime"
	"testing"
	"time"
	"weak"

	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/netease"
)

// Closing Now Playing used to keep its background canvas for the whole session.
func TestNowPlayingReleasesBackdrop(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "song", Duration: 200000}})
	a.pl.SetLyrics(nil)
	tt := ui.NewTester(a.view, 900, 600)
	a.nowPlaying = true
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	tt.Image()
	if a.backdropCache.canvas == nil {
		t.Fatal("Now Playing did not create its backdrop")
	}
	old := weak.Make(a.backdropCache.canvas)
	a.nowPlaying = false
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Image()
	if a.backdropCache.canvas != nil || a.backdropCache.before != nil {
		t.Fatal("the closed view retained its backdrop")
	}
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("the closed view's canvas is still referenced")
	}
	runtime.KeepAlive(a)
	runtime.KeepAlive(tt)
}
