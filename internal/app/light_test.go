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

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// The glow is on by default, for settings kept before it was there, and
// turning it off stays off.
func TestNpLightOffIsKept(t *testing.T) {
	s := defaultSettings()
	json.Unmarshal([]byte(`{"quality":"exhigh"}`), &s)
	if !s.NpLight {
		t.Error("settings from before the option turned the glow off")
	}
	s.NpLight = false
	b, _ := json.Marshal(s)
	back := defaultSettings()
	json.Unmarshal(b, &back)
	if back.NpLight {
		t.Error("turning the glow off did not survive being saved")
	}
}

// A short word has no swell, and a word held does only while it is sung,
// highest around its middle and gone by its end: a line left glowing is what
// went too far before.
func TestWordHeld(t *testing.T) {
	short := lyrics.Word{Start: 0, End: 900 * time.Millisecond}
	if v := wordHeld(short, true, 450*time.Millisecond); v != 0 {
		t.Errorf("a short word swells by %v", v)
	}
	long := lyrics.Word{Start: 0, End: 3 * time.Second}
	peak := wordHeld(long, false, 1500*time.Millisecond)
	if peak <= 0 || peak > 0.8 {
		t.Errorf("a word held 3 s swells by %v", peak)
	}
	if v := wordHeld(long, false, 200*time.Millisecond); v >= peak {
		t.Errorf("as it begins it swells by %v, past its middle's %v", v, peak)
	}
	if v := wordHeld(long, false, 3*time.Second); v != 0 {
		t.Errorf("sung, it still swells by %v", v)
	}
	if v := wordHeld(long, true, 1800*time.Millisecond); v <= peak {
		t.Errorf("the last word swells by %v, no more than another's %v", v, peak)
	}
}

// Paused, with the pointer away, the glow asks for no frames.
func TestGlowIdle(t *testing.T) {
	a := newTestApp()
	a.settings.NpLight = true
	a.pl.Cue([]netease.Track{{ID: 1, Name: "chaotic", Duration: 180000}})
	a.pl.SetLyrics([]lyrics.Line{{At: 0, Text: "Who knew"}, {At: time.Minute, Text: "that wanting"}})
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1100, 760)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Advance(2 * time.Second)
	if tt.FramePending() {
		t.Error("the glow keeps drawing while nothing changes")
	}
}

// With NM_FRAMES set, saves Now Playing's glow at rest, and with the pointer
// on the progress bar, the volume and the line sung, for comparing them.
func TestGlowFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	a := newTestApp()
	a.settings.NpLight = true
	a.volume = 0.6
	a.pl.Cue([]netease.Track{{ID: 1, Name: "chaotic", Duration: 180000}})
	a.pl.SetLyrics([]lyrics.Line{
		{At: 0, Text: "前一句歌词"},
		{At: 70 * time.Second, End: 76 * time.Second, Text: "谁知道这份渴望", Timed: true, Words: []lyrics.Word{
			{Text: "谁知道", Start: 70 * time.Second, End: 70800 * time.Millisecond},
			{Text: "这份", Start: 70800 * time.Millisecond, End: 71500 * time.Millisecond},
			{Text: "渴望", Start: 71500 * time.Millisecond, End: 74500 * time.Millisecond},
		}},
		{At: 80 * time.Second, Text: "后一句歌词"},
	})
	a.pl.Seek(73 * time.Second)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetDark(true)
	settle := func(name string) {
		tt.Advance(time.Second)
		tt.Frame()
		f, _ := os.Create(filepath.Join(dir, name+".png"))
		defer f.Close()
		png.Encode(f, tt.Image())
	}
	settle("glow-settle")
	settle("glow-rest")
	r, _ := tt.Find("播放进度")
	tt.Move(r.X+r.W*0.72, r.Y+r.H/2)
	settle("glow-scrub")
	v, _ := tt.Find("音量")
	tt.Move(v.X+v.W*0.35, v.Y+v.H/2)
	settle("glow-volume")
	l, _ := tt.Find("谁知道这份渴望")
	tt.Move(l.X+40, l.Y+l.H/2)
	settle("glow-lyric")
}

// Nothing glows ahead of the singer. A word's color at no alpha falls back
// to the color of the text around it, which lit the words to come in full,
// blurred, the brightest thing on the line.
func TestGlowOnlyWhereSung(t *testing.T) {
	frame := func(glow bool) *image.RGBA {
		a := newTestApp()
		a.settings.NpLight, a.settings.GlowLevel = glow, 0.5
		a.pl.Cue([]netease.Track{{ID: 1, Name: "x", Duration: 180000}})
		a.pl.SetLyrics([]lyrics.Line{{At: time.Second, End: 9 * time.Second, Text: "Why do the stars", Timed: true, Words: []lyrics.Word{
			{Text: "Why ", Start: time.Second, End: 2 * time.Second},
			{Text: "do the stars", Start: 6 * time.Second, End: 8 * time.Second},
		}}})
		a.pl.Seek(3 * time.Second)
		a.nowPlaying = true
		tt := ui.NewTester(a.view, 1100, 760)
		tt.SetDark(true)
		tt.Advance(2 * time.Second)
		tt.Frame()
		tt.Advance(2 * time.Second)
		tt.Frame()
		return tt.Image()
	}
	on, off := frame(true), frame(false)
	// The left of the pane has the word sung; the right of it, those to come.
	sum := func(img *image.RGBA, x0, x1 int) int {
		s := img.Bounds().Dx() / 1100
		n := 0
		for y := 60; y < 300; y++ {
			for x := x0; x < x1; x++ {
				c := img.RGBAAt(x*s, y*s)
				n += int(c.R)
			}
		}
		return n
	}
	const split = 560 // past "Why ", in the words to come
	if diff := sum(on, split, 900) - sum(off, split, 900); diff > 400 || diff < -400 {
		t.Errorf("the words to come changed by %d with the glow", diff)
	}
	if diff := sum(on, 470, split) - sum(off, 470, split); diff < 400 {
		t.Errorf("the word sung gained only %d with the glow", diff)
	}
}
