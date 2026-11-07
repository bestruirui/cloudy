package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/netease"
)

// The volume opens as the pointer comes over the speaker, which shows that
// the wheel there turns it, and closes a moment after it leaves; a click
// keeps it open.
func TestVolumeOpensOnHover(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "song", Duration: 200000}})
	tt := ui.NewTester(a.view, 1180, 780)
	tt.Frame()
	r, ok := tt.Find("音量")
	if !ok {
		t.Fatal("no speaker")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !a.volumeOpen {
		t.Fatal("the pointer over the speaker did not open the volume")
	}
	tt.Move(20, 400)
	tt.Frame()
	if !a.volumeOpen {
		t.Fatal("it closed at once, before the pointer could reach it")
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.volumeOpen {
		t.Fatal("it stayed open after the pointer left")
	}

	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.Move(20, 400)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	if !a.volumeOpen {
		t.Fatal("a click did not keep it open")
	}
}
