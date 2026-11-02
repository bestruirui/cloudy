package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The mini window seeks where its bar is dragged, and sets the volume where its
// slider is clicked: neither goes to the window, which drags from elsewhere.
func TestMiniSeeksAndSetsVolume(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.volume = 0.8
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	tt := ui.NewTester(a.miniView, miniW, miniH)
	tt.Frame()

	const y = miniH - 12
	left, width := float32(18), float32(miniW-36)
	tt.Press(left+width/4, y)
	tt.Frame()
	tt.Move(left+width*3/4, y)
	tt.Frame()
	tt.Release(left+width*3/4, y)
	tt.Frame()
	if got := a.pl.Position(); got < 140*time.Second || got > 160*time.Second {
		t.Errorf("dragged to three quarters of 200s, at %v", got)
	}

	r, ok := tt.Find("音量")
	if !ok {
		t.Fatal("no volume slider")
	}
	tt.ClickAt(r.X+r.W/4, r.Y+r.H/2)
	tt.Frame()
	if a.volume < 0.15 || a.volume > 0.35 {
		t.Errorf("clicked a quarter along, volume %v", a.volume)
	}
}
