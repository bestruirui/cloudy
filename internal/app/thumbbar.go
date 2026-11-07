package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo"
	"golang.org/x/image/vector"
)

// The window's taskbar thumbnail (Windows), as NetEase's own app has it:
// the cover of the song playing in place of the window, the song and its
// artists as its title, and under it the song before, play or pause, the
// next, and the heart. They follow what plays, the heart, and the
// taskbar's light or dark, whose color the glyphs take the other of.

// thumbState is what the thumbnail shows; it changes only when this does.
type thumbState struct {
	track          int64 // 0: no song, nothing shown
	title          string
	cover          bool // the cover is in, and shows
	playing, dark  bool
	liked, canLike bool
}

// syncThumbs puts the thumbnail as the player is. It is cheap when nothing
// changed, as it is called often: by every frame and every post.
func (a *app) syncThumbs() {
	if a.win == nil || a.pl == nil {
		return
	}
	var st thumbState
	if a.hasSong() {
		tr := a.pl.Current()
		st = thumbState{
			track:   tr.ID,
			title:   songTitle(tr.Name, tr.ArtistNames()),
			cover:   a.pl.Cover() != nil,
			playing: a.pl.Playing(),
			dark:    a.thumbDark,
			liked:   a.isLiked(tr),
			canLike: tr.LocalPath == "",
		}
	}
	at := a.thumbsAt
	if at != nil && *at == st {
		return
	}
	a.thumbsAt = &st
	if at == nil || at.title != st.title {
		title := st.title
		if title == "" {
			title = mygo.App.Name()
		}
		a.win.SetTitle(title)
	}
	switch {
	case st.track == 0:
		if at == nil || at.track != 0 {
			a.win.SetThumbnail(nil) // the window again
		}
	case st.cover && (at == nil || at.track != st.track || !at.cover):
		// The new cover is in. Until it is, the one before stays: going
		// back to the window in between, which shows nothing while it is
		// minimized or in the tray, left the thumbnail blank.
		a.win.SetThumbnail(a.pl.Cover())
	}
	if st.track == 0 {
		a.win.SetThumbButtons(nil)
		return
	}
	ink := color.NRGBA{0x1b, 0x1b, 0x1f, 0xff}
	if st.dark {
		ink = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	}
	play, playTip := thumbGlyph(glyphPlay, ink), "播放"
	if st.playing {
		play, playTip = thumbGlyph(glyphPause, ink), "暂停"
	}
	heart, heartTip := thumbGlyph(glyphHeart, ink), "喜欢"
	if st.liked {
		heart, heartTip = thumbGlyph(glyphHeartFull, likeRed), "取消喜欢"
	}
	act := func(fn func()) func() {
		return func() { fn(); a.syncThumbs(); a.redraw() }
	}
	a.win.SetThumbButtons([]mygo.ThumbButton{
		{Icon: thumbGlyph(glyphPrev, ink), Tooltip: "上一首", Click: act(a.pl.Prev)},
		{Icon: play, Tooltip: playTip, Click: act(a.pl.TogglePlay)},
		{Icon: thumbGlyph(glyphNext, ink), Tooltip: "下一首", Click: act(a.pl.Next)},
		{Icon: heart, Tooltip: heartTip, Disabled: !st.canLike, Click: act(a.likeCurrent)},
	})
}

// songTitle is the window's title for a song: its name and its artists,
// as NetEase's app shows it under its taskbar button.
func songTitle(name, artists string) string {
	if artists == "" {
		return name
	}
	return name + " - " + artists
}

// hasSong reports whether there is a song to play: none at the start,
// before one is chosen.
func (a *app) hasSong() bool {
	tr := a.pl.Current()
	return tr != nil && tr.ID != 0
}

// readTaskbarDark notes whether the taskbar is dark, which the glyphs take
// the other color of: the tray's icon knows, else the system's appearance
// says. It is read as the appearance changes, not by every frame.
func (a *app) readTaskbarDark() {
	if a.tray != nil {
		a.thumbDark = a.tray.Dark()
	} else {
		a.thumbDark = mygo.Theme.IsDark()
	}
	a.syncThumbs()
}

// likeRed is the color of a liked song's heart.
var likeRed = color.NRGBA{0xff, 0x3b, 0x5c, 0xff}

type glyph int

const (
	glyphPrev glyph = iota
	glyphPlay
	glyphPause
	glyphNext
	glyphHeart     // an outline: not liked
	glyphHeartFull // liked
)

// thumbSize is how many pixels across a glyph is drawn: Windows draws a
// thumbnail button's icon 16 across at 100%, 32 at 200%, from this.
const thumbSize = 48

// thumbGlyph draws g in ink, as a PNG.
func thumbGlyph(g glyph, ink color.Color) []byte {
	const s = thumbSize
	img := image.NewNRGBA(image.Rect(0, 0, s, s))
	switch g {
	case glyphHeart:
		// The heart less a smaller one inside it.
		outer, inner := heartMask(1), heartMask(0.62)
		for i := range outer.Pix {
			outer.Pix[i] = uint8(int(outer.Pix[i]) * (255 - int(inner.Pix[i])) / 255)
		}
		paint(img, outer, ink)
	case glyphHeartFull:
		paint(img, heartMask(1), ink)
	default:
		r := vector.NewRasterizer(s, s)
		poly := func(pts ...[2]float32) {
			r.MoveTo(pts[0][0]*s, pts[0][1]*s)
			for _, p := range pts[1:] {
				r.LineTo(p[0]*s, p[1]*s)
			}
			r.ClosePath()
		}
		bar := func(x0, y0, x1, y1 float32) {
			poly([2]float32{x0, y0}, [2]float32{x1, y0}, [2]float32{x1, y1}, [2]float32{x0, y1})
		}
		switch g {
		case glyphPlay:
			poly([2]float32{0.27, 0.14}, [2]float32{0.85, 0.5}, [2]float32{0.27, 0.86})
		case glyphPause:
			bar(0.22, 0.16, 0.41, 0.84)
			bar(0.59, 0.16, 0.78, 0.84)
		case glyphPrev:
			bar(0.16, 0.2, 0.28, 0.8)
			poly([2]float32{0.84, 0.2}, [2]float32{0.84, 0.8}, [2]float32{0.32, 0.5})
		case glyphNext:
			bar(0.72, 0.2, 0.84, 0.8)
			poly([2]float32{0.16, 0.2}, [2]float32{0.68, 0.5}, [2]float32{0.16, 0.8})
		}
		r.Draw(img, img.Bounds(), image.NewUniform(ink), image.Point{})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// heartMask is the coverage of a heart, scaled by k about the middle.
func heartMask(k float32) *image.Alpha {
	const s = thumbSize
	m := image.NewAlpha(image.Rect(0, 0, s, s))
	r := vector.NewRasterizer(s, s)
	p := func(x, y float32) (float32, float32) {
		return (0.5 + (x-0.5)*k) * s, (0.52 + (y-0.52)*k) * s
	}
	r.MoveTo(p(0.5, 0.88))
	cube := func(x1, y1, x2, y2, x3, y3 float32) {
		ax, ay := p(x1, y1)
		bx, by := p(x2, y2)
		cx, cy := p(x3, y3)
		r.CubeTo(ax, ay, bx, by, cx, cy)
	}
	cube(0.1, 0.6, 0.04, 0.38, 0.12, 0.24)
	cube(0.2, 0.1, 0.42, 0.1, 0.5, 0.27)
	cube(0.58, 0.1, 0.8, 0.1, 0.88, 0.24)
	cube(0.96, 0.38, 0.9, 0.6, 0.5, 0.88)
	r.ClosePath()
	r.Draw(m, m.Bounds(), image.Opaque, image.Point{})
	return m
}

// paint puts ink on img where mask covers it.
func paint(img *image.NRGBA, mask *image.Alpha, ink color.Color) {
	c := color.NRGBAModel.Convert(ink).(color.NRGBA)
	for i, a := range mask.Pix {
		if a == 0 {
			continue
		}
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2] = c.R, c.G, c.B
		img.Pix[i*4+3] = uint8(int(a) * int(c.A) / 255)
	}
}
