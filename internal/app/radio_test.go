package app

import (
	"image"
	"image/color"
	stdpalette "image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func TestCommentTime(t *testing.T) {
	now := time.Date(2026, 10, 25, 22, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{now.Add(-20 * time.Second), "刚刚"},
		{now.Add(-42 * time.Minute), "42分钟前"},
		{now.Add(-5 * time.Hour), "17:00"},
		{now.Add(-23 * time.Hour), "昨天 23:00"},
		{time.Date(2026, 3, 8, 9, 0, 0, 0, time.Local), "3月8日"},
		{time.Date(2021, 4, 10, 9, 0, 0, 0, time.Local), "2021年4月10日"},
	} {
		if got := commentTime(c.t, now); got != c.want {
			t.Errorf("%v: %q, want %q", c.t, got, c.want)
		}
	}
}

// Heart mode playing, its card and Now Playing offer to leave it, which
// keeps the song playing.
func TestExitHeartMode(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "听众"}
	a.pl.Cue([]netease.Track{{ID: 1, Name: "晚风"}, {ID: 2, Name: "推荐的歌"}})
	a.pl.AttachRadio(heartRadio, func(got func([]netease.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 || len(a.pl.Queue()) != 1 {
		t.Fatalf("radio %q, current %v, queue %d", a.pl.Radio(), a.pl.Current(), len(a.pl.Queue()))
	}
	if !tt.HasText("开启心动模式") {
		t.Errorf("card still on: %q", tt.Texts())
	}

	a.pl.AttachRadio(heartRadio, func(got func([]netease.Track)) {})
	a.nowPlaying = true
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("退出心动模式"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 {
		t.Fatalf("from Now Playing: radio %q, current %v", a.pl.Radio(), a.pl.Current())
	}
}

// The cover behind the current song is the one Next plays, even after shuffling.
func TestFMNextCoverFollowsPlayback(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}, {ID: 4, Name: "四"}})
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	a.pl.ToggleShuffle()
	tt := ui.NewTester(a.fmCard, 1000, 400)
	tt.Frame()
	for range 2 {
		next := a.pl.NextIndex()
		if next < 0 {
			t.Fatal("the radio has no next song")
		}
		want := a.pl.Queue()[next]
		if _, ok := tt.Find("下一首封面：" + want.Name); !ok {
			t.Fatalf("the cover behind does not show %s", want.Name)
		}
		if err := tt.Click("下一首"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if got := a.pl.Current(); got.ID != want.ID {
			t.Fatalf("played %s after showing %s", got.Name, want.Name)
		}
		if _, ok := tt.Find("当前封面：" + want.Name); !ok {
			t.Fatalf("the next cover did not become current: %s", want.Name)
		}
		tt.Advance(time.Second)
		tt.Frame()
	}
}

// Rapid skips and reversals keep one copy of each cover, then release the old ones.
func TestFMCoversKeepDepartingUntilFaded(t *testing.T) {
	a := newTestApp()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}, {ID: 4, Name: "四"}})
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	tt := ui.NewTester(func(c *ui.Context) { a.fmCovers(c, a.pl.Current()) }, 360, 300)
	tt.Frame()
	a.pl.Next()
	tt.Frame()
	tt.Advance(60 * time.Millisecond)
	a.pl.Next()
	tt.Frame()
	if len(a.fmCoversState.leaving) != 2 {
		t.Fatal("a quick skip dropped a cover before it faded")
	}
	a.pl.Prev()
	tt.Frame()
	for _, item := range a.fmCoversState.leaving {
		if item.track.ID == 2 || item.track.ID == 3 {
			t.Fatal("a queued cover was also drawn as departing")
		}
	}
	tt.Advance(time.Second)
	tt.Frame()
	if len(a.fmCoversState.leaving) != 0 {
		t.Fatal("the stack kept faded covers")
	}
}

// A backdrop arriving mid-handoff must not recreate the moving cover's parents.
func TestFMCoverKeepsMovingWhenBackdropLoads(t *testing.T) {
	for _, tc := range []struct {
		name     string
		oldReady bool
		arrives  time.Duration
	}{
		{"warm-40ms", true, 40 * time.Millisecond},
		{"warm-80ms", true, 80 * time.Millisecond},
		{"warm-160ms", true, 160 * time.Millisecond},
		{"cold-40ms", false, 40 * time.Millisecond},
		{"cold-80ms", false, 80 * time.Millisecond},
		{"cold-160ms", false, 160 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			tracks := []netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}}
			for i := range tracks {
				tracks[i].Album.PicURL = "https://example.com/fm" + tracks[i].Name
				markArt(a, tracks[i].Album.PicURL, 600)
				a.art.busy[netease.Sized(tracks[i].Album.PicURL, 600)+"#backdrop"] = true
			}
			if tc.oldReady {
				first := a.art.peek(netease.Sized(tracks[0].Album.PicURL, 600))
				a.art.add(netease.Sized(tracks[0].Album.PicURL, 600)+"#backdrop", &artwork{backdrop: first.bmp})
			}
			a.pl.Cue(tracks)
			a.pl.Hold(0)
			a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
			tt := ui.NewTester(a.fmCard, 1000, 400)
			tt.Frame()
			a.pl.Next()
			tt.Frame()
			tt.Advance(tc.arrives)
			tt.Frame()
			before, ok := tt.Find("当前封面：二")
			if !ok || before.X <= 45 {
				t.Fatalf("the next cover did not start moving forward: %+v", before)
			}
			second := a.art.peek(netease.Sized(tracks[1].Album.PicURL, 600))
			a.art.add(netease.Sized(tracks[1].Album.PicURL, 600)+"#backdrop", &artwork{backdrop: second.bmp})
			tt.Frame()
			tt.Advance(time.Millisecond)
			tt.Frame()
			after, ok := tt.Find("当前封面：二")
			if !ok || math.Abs(float64(after.X-before.X)) > 1 || math.Abs(float64(after.W-before.W)) > 1 {
				t.Fatalf("the backdrop made the cover jump in 1 ms: before %+v, after %+v", before, after)
			}
		})
	}
}

// With NM_FRAMES set, saves the FM stack before and during a change of song.
func TestFMCoverFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	tracks := []netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}}
	colors := []color.RGBA{{220, 65, 65, 255}, {65, 145, 220, 255}, {75, 190, 120, 255}}
	for i := range tracks {
		tracks[i].Album.PicURL = "https://example.com/cover" + tracks[i].Name
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := range 64 {
			for x := range 64 {
				img.SetRGBA(x, y, colors[i])
			}
		}
		a.art.add(netease.Sized(tracks[i].Album.PicURL, 600), &artwork{bmp: ui.NewBitmap(img), used: time.Now(), size: 1})
	}
	a.pl.Cue(tracks)
	a.pl.Hold(0)
	a.pl.AttachRadio(fmRadio(netease.FMDefault), func(func([]netease.Track)) {})
	tt := ui.NewTester(a.fmCard, 1000, 400)
	tt.Frame()
	tt.Advance(time.Second)
	for i, elapsed := range []time.Duration{0, 60 * time.Millisecond, 140 * time.Millisecond, 500 * time.Millisecond} {
		if i == 1 {
			a.pl.Next()
			tt.Frame()
		}
		tt.Advance(elapsed)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, []string{"fm-before.png", "fm-60ms.png", "fm-200ms.png", "fm-rest.png"}[i]))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	// A separate, evenly sampled run shows the cadence between the stills.
	a.pl.Cue(tracks)
	a.pl.Hold(0)
	a.fmCoversState = fmCoverStack{}
	tt = ui.NewTester(a.fmCard, 1000, 400)
	tt.Frame()
	tt.Advance(time.Second)
	animation := &gif.GIF{LoopCount: 0}
	for frame := range 40 {
		if frame == 6 {
			a.pl.Next()
			tt.Frame()
		}
		tt.Advance(40 * time.Millisecond)
		tt.Frame()
		img := tt.Image().SubImage(image.Rect(0, 15, 325, 305))
		paletted := image.NewPaletted(image.Rect(0, 0, 325, 290), stdpalette.Plan9)
		draw.FloydSteinberg.Draw(paletted, paletted.Bounds(), img, img.Bounds().Min)
		animation.Image = append(animation.Image, paletted)
		animation.Delay = append(animation.Delay, 4)
	}
	f, err := os.Create(filepath.Join(dir, "fm-smooth.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, animation); err != nil {
		t.Fatal(err)
	}
}
