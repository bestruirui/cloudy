package app

import (
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The app draws the content on paper when the listener chooses it: a sheet a
// breath warmer at its top than at its foot, a lamp over that top, and fiber
// in it, which is what makes it read as paper rather than as a beige window.
// The sidebar keeps its own color, as the page of a book keeps its cover.

// paper is a sheet the content may be drawn on: its colors and its fiber.
type paper struct {
	top, bottom ui.Color // the sheet, from its top to its foot
	lamp        ui.Color // the light over its top, alpha and all
	fiber       float32  // how strong its fiber shows, 0 for none
	fiberSize   float32  // how large the fiber is, 1 for a pixel
}

// paperSheets are the sheets the listener may choose, the parchment for now,
// in the light and in the dark, as the dots that choose them show them.
var paperSheets = []struct {
	id, label   string
	light, dark ui.Color
}{
	{"parchment", "羊皮纸", ui.Hex("#F4EFE1"), ui.Hex("#33302A")},
}

// sheet returns the paper to draw on, or nothing where the listener turned it
// off. The fiber follows the texture they chose: fine, soft or coarse.
func (s settings) sheet(dark bool) paper {
	if !s.Paper {
		return paper{}
	}
	var p paper
	if dark {
		p = paper{
			top:    ui.Hex("#2B2926"),
			bottom: ui.Hex("#232220"),
			lamp:   ui.Hex("#3B3225").Alpha(0.10),
			fiber:  0.06,
		}
	} else {
		p = paper{
			top:    ui.Hex("#FAF7EF"),
			bottom: ui.Hex("#F3EFE3"),
			lamp:   ui.Hex("#E9D9B4").Alpha(0.30),
			fiber:  0.07,
		}
	}
	p.fiberSize = 1
	switch s.PaperTexture {
	case "soft":
		p.fiber *= 0.9
		p.fiberSize = 2
	case "coarse":
		p.fiber *= 1.15
		p.fiberSize = 3
	}
	return p
}

// paperInk returns the colors the content and its words take on the sheet,
// warmer than the flat background's, which the sidebar keeps.
func paperInk(p palette) palette {
	if p.dark {
		p.bg = ui.Hex("#2B2926")
		p.text = ui.Hex("#F5F4ED")
		p.muted = ui.Hex("#B0AEA5")
		p.faint = ui.Hex("#87867F")
		p.hairline = ui.RGBA(250, 249, 245, 0.09)
		p.hover = ui.RGBA(250, 249, 245, 0.05)
		p.press = ui.RGBA(250, 249, 245, 0.09)
		p.cardShade = ui.RGBA(0, 0, 0, 0.5)
		return p
	}
	p.bg = ui.Hex("#FAF7EF")
	p.text = ui.Hex("#141413")
	p.muted = ui.Hex("#5E5D59")
	p.faint = ui.Hex("#87867F")
	p.hairline = ui.RGBA(31, 30, 29, 0.12)
	p.hover = ui.RGBA(115, 114, 108, 0.1)
	p.press = ui.RGBA(115, 114, 108, 0.17)
	p.cardShade = ui.RGBA(74, 58, 32, 0.14)
	return p
}

// paperFiber returns the two halves of the fiber, the dark specks and the
// light ones, which over the sheet make its grain: a pixel is a speck of one
// or of the other, never of both.
var paperFiber struct {
	once        sync.Once
	dark, light *ui.Bitmap
}

func fiberBitmaps() (dark, light *ui.Bitmap) {
	paperFiber.once.Do(func() {
		const n = 512
		d, l := image.NewRGBA(image.Rect(0, 0, n, n)), image.NewRGBA(image.Rect(0, 0, n, n))
		x := uint32(0x9e3779b9)
		for i := range n * n {
			// A xorshift, so that the grain is the same every run and in
			// every frame, with no table to keep.
			x ^= x << 13
			x ^= x >> 17
			x ^= x << 5
			v := uint8(x >> 24)
			at := i * 4
			d.Pix[at], d.Pix[at+1], d.Pix[at+2], d.Pix[at+3] = 255, 255, 255, 255-v
			l.Pix[at], l.Pix[at+1], l.Pix[at+2], l.Pix[at+3] = 255, 255, 255, v
		}
		paperFiber.dark, paperFiber.light = ui.NewBitmap(d), ui.NewBitmap(l)
	})
	return paperFiber.dark, paperFiber.light
}

// paperLamp is the light over the top of the sheet: a wide, soft falloff,
// stretched over the top of the content so that it reads as one lamp.
var paperLamp struct {
	once sync.Once
	bmp  *ui.Bitmap
}

func lampBitmap() *ui.Bitmap {
	paperLamp.once.Do(func() {
		const n = 512
		img := image.NewRGBA(image.Rect(0, 0, n, n))
		for y := range n {
			for x := range n {
				dx := (float64(x) - n/2) / (n / 2)
				dy := (float64(y) - n/2) / (n / 2)
				d := math.Sqrt(dx*dx + dy*dy)
				a := math.Max(0, 1-d)
				img.SetRGBA(x, y, color.RGBA{A: uint8(a * a * 255)})
			}
		}
		paperLamp.bmp = ui.NewBitmap(img)
	})
	return paperLamp.bmp
}

// paperBackdrop draws the sheet over el, the box of the content, under what
// the page and its bars draw over it.
func (a *app) paperBackdrop(el ui.Element, p palette) {
	sheet := a.settings.sheet(p.dark)
	if sheet.fiber == 0 && sheet.lamp.A == 0 {
		el.Background(p.bg)
		return
	}
	el.Background(sheet.top)
	el.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.FillGradient(r, ui.LinearGradient{Angle: 180, Oklab: true, From: sheet.top, To: sheet.bottom}, 0)
		pt.ImageRegion(lampBitmap(), ui.Rect{}, ui.Rect{
			X: r.X - r.W*0.3,
			Y: r.Y - r.H*0.7,
			W: r.W * 1.6,
			H: r.H * 1.4,
		}, ui.ImageOptions{Tint: sheet.lamp})
		a.paperFiber(pt, r, sheet)
	})
}

// paperFiber draws the fiber of the sheet over r, tiles of grain the size of
// the texture chosen, dark specks then light ones.
func (a *app) paperFiber(pt *ui.Painter, r ui.Rect, sheet paper) {
	dark, light := fiberBitmaps()
	size := 512 * sheet.fiberSize
	x0 := float32(math.Floor(float64(r.X)))
	y0 := float32(math.Floor(float64(r.Y)))
	darkTint := ui.Hex("#4A3A26").Alpha(sheet.fiber)
	lightTint := ui.Hex("#FFFDF5").Alpha(sheet.fiber * 0.85)
	for y := y0; y < r.Y+r.H; y += size {
		for x := x0; x < r.X+r.W; x += size {
			tile := ui.Rect{X: x, Y: y, W: size, H: size}
			pt.ImageRegion(dark, ui.Rect{}, tile, ui.ImageOptions{Tint: darkTint})
			pt.ImageRegion(light, ui.Rect{}, tile, ui.ImageOptions{Tint: lightTint})
		}
	}
}
