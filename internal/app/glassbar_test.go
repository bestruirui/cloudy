package app

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The bar of glass is on by default, for settings kept before it was there,
// and turning it off stays off: dropped from the file, as omitempty would
// have false, it would load as the default again.
func TestLiquidGlassOffIsKept(t *testing.T) {
	s := defaultSettings()
	json.Unmarshal([]byte(`{"quality":"exhigh"}`), &s)
	if !s.LiquidGlass {
		t.Error("settings from before the option turned the glass off")
	}
	s.LiquidGlass = false
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	back := defaultSettings()
	json.Unmarshal(b, &back)
	if back.LiquidGlass {
		t.Error("turning the glass off did not survive being saved")
	}
}

// The capsule leaves the room beside it to the page, whose columns reach to
// the window's edge, and the bar on glass opens Now Playing as the one along
// the foot does.
func TestGlassBarOpensNowPlaying(t *testing.T) {
	a := newTestApp()
	a.settings.LiquidGlass = true
	a.pl.Cue([]netease.Track{{ID: 1, Name: "Echo of You"}})
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	if err := tt.Click("打开正在播放"); err != nil {
		t.Fatal(err)
	}
	if !a.nowPlaying {
		t.Error("the cover on the glass did not open Now Playing")
	}
}

// Paused, the bar of glass asks for no frames: each of them would read and
// blur what is under the pane again.
func TestGlassBarIdle(t *testing.T) {
	a := newTestApp()
	a.settings.LiquidGlass = true
	a.pl.Cue([]netease.Track{{ID: 1, Name: "Echo of You", Duration: 125000}})
	tt := ui.NewTester(a.view, 1100, 700)
	tt.Frame()
	tt.Advance(2 * time.Second)
	if tt.FramePending() {
		t.Error("the bar of glass keeps drawing while nothing changes")
	}
}

// With NM_FRAMES set, saves the foot of the window with the bar of glass,
// over a page of covers, in the light and in the dark.
func TestGlassBarFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		a.settings.LiquidGlass = true
		a.settings.Smoke = true
		tr := netease.Track{ID: 1, Name: "Echo of You", Duration: 125000}
		a.pl.Cue([]netease.Track{tr})
		a.pl.Seek(25 * time.Second)
		tt := ui.NewTester(a.view, 1180, 700)
		tt.SetDark(dark)
		tt.Frame()
		img := tt.Image()
		s := float32(img.Bounds().Dx()) / 1180
		name := "glassbar-light.png"
		if dark {
			name = "glassbar-dark.png"
		}
		sub := img.SubImage(image.Rect(0, int(500*s), img.Bounds().Dx(), img.Bounds().Dy()))
		f, _ := os.Create(filepath.Join(dir, name))
		png.Encode(f, sub)
		f.Close()
	}
}

// The bar's own glass is compiled the first time a window shows it on a GPU,
// a panic there: a mistake in it must show here.
func TestBarGlassCompiles(t *testing.T) {
	barGlassShader()
}
