package app

import (
	"bytes"
	_ "embed"
	"image"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Where the app's source is, as the AGPL asks of a modified version, and what
// it was made from.
const (
	appSource = "https://github.com/nya-owo/cloudy"
	appOrigin = "https://github.com/egoist/mygo-cloudmusic"
)

//go:embed brand/mascot.png
var mascotPNG []byte

//go:embed brand/icon.png
var brandPNG []byte

func embeddedBitmap(b []byte) func() *ui.Bitmap {
	return sync.OnceValue(func() *ui.Bitmap {
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			log.Printf("brand: %v", err)
			return nil
		}
		return ui.NewBitmap(img)
	})
}

// The little dinosaurs of the record on the app's icon, and the icon.
var (
	mascot    = embeddedBitmap(mascotPNG)
	brandIcon = embeddedBitmap(brandPNG)
)

// mascotView is who shows where a page has nothing to show.
func mascotView(c *ui.Context, size float32) ui.Element {
	return ui.Image(c, mascot()).Size(size, size).Fit(ui.Cover)
}

// aboutCard says what the app is, whose it is and where its source is.
func (a *app) aboutCard(k *settingsKit) {
	c, ink := k.c, k.ink
	ui.Row(c).Padding(16, 20).Gap(14).AlignItems(ui.Center).Children(func() {
		ui.Image(c, brandIcon()).Size(52, 52).Fit(ui.Cover).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, mygo.App.Name()+" "+mygo.App.Version()).FontSize(14).FontWeight(700)
			ui.Text(c, "由 nya 制作，基于 egoist/mygo-cloudmusic 修改，以 AGPL-3.0 开源。与网易云音乐官方无关").
				FontSize(12).TextColor(ink.muted).LineHeight(1.4)
		})
		if k.button("source", "源码", iconExternal, false, false).Clicked() {
			openLink(appSource)
		}
		if k.button("origin", "原项目", iconExternal, false, false).Clicked() {
			openLink(appOrigin)
		}
	})
}

// openLink opens a page in the browser, off the frame.
func openLink(url string) {
	go func() {
		if err := mygo.Shell.OpenExternal(url); err != nil {
			log.Printf("open %s: %v", url, err)
		}
	}()
}

// How the record comes in as the app starts.
const (
	splashTime = 1150 * time.Millisecond
	splashDisc = 168
)

// splash is a record over the window as the app starts, spinning up out of
// nothing and fading into the app, whose first page is loading under it. It
// takes no clicks, for nothing to wait on it.
func (a *app) splash(c *ui.Context) {
	if a.splashed {
		return
	}
	if a.motion != motionFull {
		a.splashed = true
		return
	}
	now := c.Now()
	if a.splashAt.IsZero() {
		a.splashAt = now
	}
	t := float32(now.Sub(a.splashAt)) / float32(splashTime)
	if t >= 1 {
		a.splashed = true
		return
	}
	c.AnimationFrame()
	in := 1 - easeRange(1-t, 0.45, 1)
	gone := easeRange(t, 0.68, 1)
	s := lerp(0.72, 1, in) + 0.06*gone
	ui.Box(c.Key("splash")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Center().
		Background(a.pal.bg).Opacity(1 - gone).Children(func() {
		disc := ui.Box(c.Key("disc")).Size(splashDisc, splashDisc).Radius(999).Center().PassThrough().
			Background(ui.Hex("#232327")).Border(1, white.Alpha(0.14)).Shadow(0, 16, 44, -10, ui.RGBA(0, 0, 0, 0.5)).
			Opacity(easeRange(t, 0, 0.3)).Scale(s, s).Rotate(lerp(-150, 0, in))
		disc.Draw(func(p *ui.Painter, r ui.Rect) {
			// Its grooves, and the light across them.
			for i := float32(0); i < 5; i++ {
				inset := 8 + i*6
				p.Stroke(ui.Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}, white.Alpha(0.09), (r.W-2*inset)/2, 1)
			}
		})
		disc.Children(func() { mascotView(c, 76) })
	})
}
