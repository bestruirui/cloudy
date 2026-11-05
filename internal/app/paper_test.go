package app

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The paper is on for settings kept before it was there, and turning it off,
// or choosing its texture, stays as it was left.
func TestPaperKept(t *testing.T) {
	s := defaultSettings()
	json.Unmarshal([]byte(`{"quality":"exhigh"}`), &s)
	if !s.Paper {
		t.Error("settings from before the paper turned it off")
	}
	s.Paper, s.PaperTexture = false, "coarse"
	b, _ := json.Marshal(s)
	back := defaultSettings()
	json.Unmarshal(b, &back)
	if back.Paper {
		t.Error("turning the paper off did not survive being saved")
	}
	if back.PaperTexture != "coarse" {
		t.Errorf("the texture saved as %q", back.PaperTexture)
	}
}

// paperHome is a home page with something on it, for the frames below.
func paperHome(a *app) {
	a.settings = defaultSettings()
	a.daily = []netease.Track{
		{ID: 1, Name: "第一首", Duration: 180000, Album: netease.Album{Name: "专辑一"}},
		{ID: 2, Name: "第二首", Duration: 240000, Album: netease.Album{Name: "专辑二"}},
	}
	for i := range 8 {
		a.recPlaylists = append(a.recPlaylists, netease.Playlist{ID: int64(10 + i), Name: fmt.Sprintf("推荐歌单 %d", i)})
	}
	a.charts = []netease.Playlist{{ID: 99, Name: "新歌榜"}}
}

// The paper covers the content, where without it the flat background shows.
func TestPaperCoversContent(t *testing.T) {
	frame := func(on bool) *image.RGBA {
		a := newTestApp()
		paperHome(a)
		a.settings.Paper = on
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(true)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		return tt.Image()
	}
	with, without := frame(true), frame(false)
	// A point of the content, under the cards and over the player bar.
	c1, c2 := with.RGBAAt(900, 600), without.RGBAAt(900, 600)
	if c1 == c2 {
		t.Fatal("the paper painted nothing over the content")
	}
	if c1.R <= c2.R {
		t.Errorf("the paper is not lighter than the background: %v over %v", c1, c2)
	}
}

// With NM_FRAMES set, saves the home page on paper and off, in the light and
// the dark, with each texture, and the settings that choose them, for looking
// at them.
func TestPaperFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	save := func(name string, tt *ui.Tester) {
		tt.Advance(time.Second)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	home := func(dark, on bool, texture string) *ui.Tester {
		a := newTestApp()
		paperHome(a)
		a.settings.Paper, a.settings.PaperTexture = on, texture
		tt := ui.NewTester(a.view, 1280, 820)
		tt.SetDark(dark)
		return tt
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		save("paper-"+appearance, home(dark, true, ""))
		save("flat-"+appearance, home(dark, false, ""))
		save("settings-"+appearance, func() *ui.Tester {
			a := newTestApp()
			paperHome(a)
			a.settingsOpen, a.settingsTab = true, settingsAppearance
			tt := ui.NewTester(a.view, 1280, 820)
			tt.SetDark(dark)
			return tt
		}())
	}
	save("paper-soft", home(true, true, "soft"))
	save("paper-coarse", home(false, true, "coarse"))
}
