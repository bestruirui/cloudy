package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The details of what plays open the effects; a preset turns them on with
// its bands, and moving a band makes them the listener's own.
func TestEffectsPane(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "song", Duration: 200000}})
	tt := ui.NewTester(a.view, 1180, 780)
	tt.Frame()
	if err := chooseItem(tt, "歌曲详情", "音效"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.effects.open {
		t.Fatal("the effects did not open")
	}
	if err := tt.Click("摇滚"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e := a.settings.Effects
	if !e.On || a.settings.EffectsPreset != "rock" || e.Bands[0] != 4 {
		t.Fatalf("rock: %+v, preset %q", e, a.settings.EffectsPreset)
	}
	if err := tt.Click("自定义"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, ok := tt.Find("1kHz")
	if !ok {
		t.Fatal("no 1 kHz band")
	}
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Move(r.X+r.W/2, r.Y+10)
	tt.Frame()
	tt.Release(r.X+r.W/2, r.Y+10)
	tt.Frame()
	if a.settings.EffectsPreset != customPreset || a.settings.Effects.Bands[5] <= 0 {
		t.Errorf("after moving 1 kHz up: preset %q, gain %v", a.settings.EffectsPreset, a.settings.Effects.Bands[5])
	}
}

// The pane, light and dark: NM_FRAMES saves them.
func TestEffectsFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for i, dark := range []bool{false, true} {
		for page := range 2 {
			a := newTestApp()
			a.settings = defaultSettings()
			a.usePreset(effectPresets[2])
			a.settings.Effects.Bands[5] = 3.5
			a.settings.EffectsPreset = customPreset
			a.effects.open = true
			a.effects.page = page
			tt := ui.NewTester(a.view, 1180, 780)
			tt.SetDark(dark)
			tt.Frame()
			tt.Advance(time.Second)
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("effects-%t-%d.png", dark, page)))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, tt.Image())
			f.Close()
		}
		_ = i
	}
}
