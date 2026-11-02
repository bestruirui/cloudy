package app

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
)

// Windows of their own, over the others: the lyrics on the desktop, and the
// player in small, for when the main window is out of the way.

const (
	deskW, deskH = 920, 150
	miniW, miniH = 420, 116
	miniDisc     = 72 // the cover in the mini window
)

// spot returns where the lyrics go: where they were left, or at the foot of
// the screen, centered.
func spot(x, y, w, h, foot int) (int, int) {
	if x != 0 || y != 0 {
		return x, y
	}
	wa := mygo.Screen.PrimaryDisplay().WorkArea
	return wa.X + (wa.Width-w)/2, wa.Y + wa.Height - h - foot
}

// miniSpot returns where the mini window goes: where it was left, or in the
// corner, clear of the lyrics.
func miniSpot(x, y int) (int, int) {
	if x != 0 || y != 0 {
		return x, y
	}
	wa := mygo.Screen.PrimaryDisplay().WorkArea
	return wa.X + wa.Width - miniW - 24, wa.Y + wa.Height - miniH - 24
}

// floatWindow opens a window of Cloud-nya's that floats over the others, on
// nothing, its corners its content's. On the tray's icon if it has one, as a
// flyout: on Windows only those show what is behind what native UI leaves
// clear, and they never take the keyboard from the app in use. It closes only
// as the app quits: closed, as by a key, it goes as off says.
func (a *app) floatWindow(title string, x, y, w, h int, view func(*ui.Context), off func()) *mygo.Window {
	var win *mygo.Window
	if a.tray != nil {
		win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: w, Height: h, Content: ui.View(view)})
	} else {
		win = mygo.NewWindow(mygo.WindowOptions{
			Title: mygo.App.Name() + title, Panel: true, Hidden: true, Frameless: true, Transparent: true,
			DisableShadow: true, DisableResize: true, X: x, Y: y, Width: w, Height: h, Content: ui.View(view),
		})
	}
	win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			off()
			a.redraw()
		}
	})
	return win
}

// applyDeskLyrics shows the lyrics on the desktop or hides them, as the
// settings say. Locked, the pointer goes through them to what is under them.
func (a *app) applyDeskLyrics() {
	s := &a.settings
	if a.win == nil {
		return
	}
	if s.DeskLyrics && a.deskWin == nil {
		x, y := spot(s.DeskX, s.DeskY, deskW, deskH, 40)
		a.deskWin = a.floatWindow(" 桌面歌词", x, y, deskW, deskH, a.deskView, func() {
			s.DeskLyrics = false
			a.applyDeskLyrics()
		})
		a.deskWin.OnMove(func() {
			if a.deskWin.IsVisible() {
				b := a.deskWin.Bounds()
				s.DeskX, s.DeskY = b.X, b.Y
			}
		})
	}
	if a.deskWin == nil {
		return
	}
	a.deskWin.SetIgnoreMouseEvents(s.DeskLyrics && s.DeskLock)
	if s.DeskLyrics {
		a.deskWin.Show()
		// A flyout shows by its anchor: from there, where it was left, and
		// above all, which it isn't by itself once another app comes up.
		a.deskWin.SetPosition(spot(s.DeskX, s.DeskY, deskW, deskH, 40))
		a.deskWin.SetAlwaysOnTop(true)
	} else {
		a.deskWin.Hide()
	}
}

// toggleDeskLyrics turns them on or off, from wherever that is offered.
func (a *app) toggleDeskLyrics() {
	a.settings.DeskLyrics = !a.settings.DeskLyrics
	a.applyDeskLyrics()
	a.changed()
}

// toggleDeskLock locks them where they are, or lets them be moved again.
func (a *app) toggleDeskLock() {
	a.settings.DeskLock = !a.settings.DeskLock
	a.applyDeskLyrics()
	a.changed()
}

// floatTheme gives a floating window the app's font, on nothing.
func (a *app) floatTheme(c *ui.Context) {
	t := *c.Theme()
	t.Font = a.fontFamily()
	c.SetTheme(&t)
	c.Root().Background(ui.Transparent)
}

// How lyrics can look on the desktop. Whatever is behind them, they have to
// read: the first two on a surface of their own, and the two outlined by an
// edge as sharp as their letters, for a soft shadow is a smear on white.
var deskStyles = []struct{ id, label string }{
	{"glass", "深色胶囊"},
	{"light", "浅色胶囊"},
	{"outline", "白字描边"},
	{"color", "彩色描边"},
	{"plain", "无背景"},
}

// A deskLook is the colors of a style: what they lie on, if anything, the
// words sung and to come, the line under them, and their edge, if any.
type deskLook struct {
	pill, sung, unsung, second, edge ui.Color
}

// deskLookOf returns the colors of a style, in the color of what plays.
func (a *app) deskLookOf(id string) deskLook {
	lamp := a.pal.accent
	if accent, ok := a.coverAccent(); ok {
		lamp = accent
	}
	switch id {
	case "light":
		return deskLook{pill: ui.RGBA(250, 250, 252, 0.94), sung: lamp.Mix(ui.RGB(0, 0, 0), 0.25),
			unsung: ui.RGBA(20, 20, 24, 0.48), second: ui.RGBA(20, 20, 24, 0.6)}
	case "outline":
		return deskLook{sung: lamp.Mix(white, 0.45), unsung: white, second: white, edge: ui.RGBA(0, 0, 0, 0.82)}
	case "color":
		return deskLook{sung: lamp.Mix(ui.RGB(0, 0, 0), 0.1), unsung: lamp.Mix(white, 0.55), second: lamp.Mix(ui.RGB(0, 0, 0), 0.2), edge: white}
	case "plain":
		return deskLook{sung: lamp.Mix(white, 0.45), unsung: white.Alpha(0.62), second: white.Alpha(0.7)}
	}
	return deskLook{pill: ui.RGBA(16, 16, 20, 0.8), sung: lamp.Mix(white, 0.55), unsung: white.Alpha(0.5), second: white.Alpha(0.62)}
}

// deskView is the line playing, and what follows it or says it in Chinese.
// Over the pointer it shows it can be moved, and its tools.
func (a *app) deskView(c *ui.Context) {
	a.floatTheme(c)
	a.followCover()
	s, pl := &a.settings, a.pl
	lines, _ := pl.Lyrics()
	pos := a.lyricPos()
	cur := lyrics.Index(lines, pos)
	scale := float32(1)
	if s.DeskScale > 0 {
		scale = float32(s.DeskScale)
	}
	look := a.deskLookOf(s.DeskStyle)
	root := ui.Column(c.Key("desk")).Fill().Center().PaddingX(24)
	over := root.Hovered()
	if over && !s.DeskLock {
		root.Cursor(ui.CursorGrab)
	}
	root.DragWindow().Children(func() {
		body := ui.Column(c.Key("body")).Gap(3).AlignItems(ui.Center).DragWindow()
		if look.pill.A > 0 {
			body.Padding(10, 28).Radius(26).Background(look.pill)
		}
		body.Children(func() {
			tr := pl.Current()
			switch {
			case tr == nil:
				a.deskLine(c, "main", "Cloud-nya", nil, pos, 30*scale, look, true)
			case len(lines) == 0 || cur < 0:
				a.deskLine(c, "main", tr.Name, nil, pos, 30*scale, look, true)
			default:
				l := &lines[cur]
				a.deskLine(c, "main", l.Text, l.Words, pos, 32*scale, look, true)
				switch {
				case l.Translation != "":
					a.deskLine(c, "next", l.Translation, nil, pos, 19*scale, look, false)
				case cur+1 < len(lines):
					a.deskLine(c, "next", lines[cur+1].Text, nil, pos, 19*scale, look, false)
				}
			}
		})
		if over && !s.DeskLock {
			a.deskTools(c)
		}
	})
	if pl.Playing() && len(lines) > 0 {
		c.After(40 * time.Millisecond)
	}
}

// deskLine is a line of the lyrics, lit word by word as it is sung if it is
// the main one, with an edge of eight copies of it around it if the style has
// one: there is no stroke to give text.
func (a *app) deskLine(c *ui.Context, key, text string, words []lyrics.Word, pos time.Duration, size float32, look deskLook, main bool) {
	style := func(e ui.Element) ui.Element {
		weight := 700
		if main {
			weight = 800
		}
		return e.FontSize(size).FontWeight(weight).SingleLine().MaxWidth(deskW - 90)
	}
	ui.Box(c.Key(key)).Shrink(0).PassThrough().Children(func() {
		// Positioned elements paint over those in the flow, in order: the
		// line itself is positioned too, last, and this holds its place.
		style(ui.Text(c.Key("room"), text)).Opacity(0)
		if look.edge.A > 0 {
			width := max(1.1, size*0.055)
			for i := range 8 {
				turn := float64(i) * math.Pi / 4
				style(ui.Text(c.Key(i), text)).Absolute().TextColor(look.edge).
					Left(width * float32(math.Cos(turn))).Top(width * float32(math.Sin(turn)))
			}
		}
		var line ui.Element
		switch {
		case main && len(words) > 0:
			line = style(ui.RichText(c.Key("words"))).Children(func() {
				for i, wd := range words {
					ui.Text(c.Key(i), wd.Text).TextColor(look.unsung.Mix(look.sung, wordLit(wd, pos)))
				}
			})
		case main:
			line = style(ui.Text(c.Key("line"), text)).TextColor(look.sung)
		default:
			line = style(ui.Text(c.Key("line"), text)).TextColor(look.second)
		}
		line.Absolute().Left(0).Top(0)
	})
}

// deskTools change the lyrics' look and size, lock them where they are, and
// close them, on a surface of their own, to read over any style.
func (a *app) deskTools(c *ui.Context) {
	s := &a.settings
	size := func(by float64) {
		if s.DeskScale == 0 {
			s.DeskScale = 1
		}
		s.DeskScale = math.Max(0.7, math.Min(s.DeskScale+by, 1.3))
		a.changed()
	}
	ui.Row(c.Key("tools")).Absolute().Top(2).Right(6).Gap(1).Padding(2).Radius(14).AlignItems(ui.Center).
		Background(ui.RGBA(16, 16, 20, 0.82)).Children(func() {
		chip := func(key, label, says string, wide float32, do func()) {
			b := ui.ButtonBase(c.Key(key)).Size(wide, 24).Radius(8).Center().FocusRing(false).Label(says)
			if b.Hovered() {
				b.Background(white.Alpha(0.14))
			}
			b.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(white.Alpha(0.9)) })
			if b.Clicked() {
				do()
			}
		}
		chip("look", "样式", "换一种样式", 40, func() {
			next := 0
			for i, d := range deskStyles {
				if d.id == s.DeskStyle {
					next = (i + 1) % len(deskStyles)
				}
			}
			if s.DeskStyle == "" {
				next = 1
			}
			s.DeskStyle = deskStyles[next].id
			a.changed()
			a.toast = "桌面歌词：" + deskStyles[next].label
		})
		chip("smaller", "A−", "歌词字号减小", 28, func() { size(-0.1) })
		chip("larger", "A+", "歌词字号增大", 28, func() { size(0.1) })
		if iconButton(c.Key("lock"), iconLock, 13, white.Alpha(0.9), "锁定桌面歌词").Size(28, 24).Clicked() {
			s.DeskLock = true
			a.applyDeskLyrics()
			a.changed()
			a.toast = "桌面歌词已锁定，可在设置或托盘菜单解锁"
		}
		if iconButton(c.Key("close"), iconX, 13, white.Alpha(0.9), "关闭桌面歌词").Size(28, 24).Clicked() {
			a.toggleDeskLyrics()
		}
	})
}

// miniSurface is what the mini window is made of: dark, in the cover's color.
func (a *app) miniSurface() ui.Color {
	tint := ui.RGB(70, 70, 78)
	if art := a.npArt; art != nil && !art.failed {
		tint = art.tint
	}
	return ui.Hex("#141418").Mix(tint, 0.3)
}

// How the player goes between the bar of the main window and the mini window.
const morphTime = 460 * time.Millisecond

// A floatMorph is the player on its way: the bar's capsule and its cover, going
// over the screen, which is what shows the small window is the bar of the large
// one. It is drawn on a clear window as large as its way.
type floatMorph struct {
	win       *mygo.Window
	toMini    bool
	box, disc [2]ui.Rect // from and to, in that window
	corner    [2]float32 // the capsule's, from and to
	at        time.Time
	handed    bool // the mini window has come in, or gone
	ending    int  // frames since it arrived, to leave the mini window showing
}

// barOnScreen returns where the bar of the main window and its cover are, on
// the screen.
func (a *app) barOnScreen() (bar, cover ui.Rect) {
	cb := a.win.ContentBounds()
	at := func(r ui.Rect) ui.Rect {
		return ui.Rect{X: float32(cb.X) + r.X, Y: float32(cb.Y) + r.Y, W: r.W, H: r.H}
	}
	return at(a.barBox), at(a.barSpot.cover)
}

// startMorph sends the player between the bar and the mini window at x and y,
// and reports whether it could: otherwise it is there at once.
func (a *app) startMorph(x, y int, toMini bool) bool {
	bar, cover := a.barOnScreen()
	if a.motion != motionFull || a.tray == nil || a.morph.win != nil || bar.W == 0 {
		return false
	}
	mini := ui.Rect{X: float32(x), Y: float32(y), W: miniW, H: miniH}
	disc := ui.Rect{X: mini.X + 14, Y: mini.Y + 12, W: miniDisc, H: miniDisc}
	left := int(min(bar.X, mini.X)) - 8
	top := int(min(bar.Y, mini.Y)) - 8
	w := int(max(bar.X+bar.W, mini.X+mini.W)) + 8 - left
	h := int(max(bar.Y+bar.H, mini.Y+mini.H)) + 8 - top
	in := func(r ui.Rect) ui.Rect {
		return ui.Rect{X: r.X - float32(left), Y: r.Y - float32(top), W: r.W, H: r.H}
	}
	m := &a.morph
	*m = floatMorph{toMini: toMini}
	m.box, m.disc, m.corner = [2]ui.Rect{in(bar), in(mini)}, [2]ui.Rect{in(cover), in(disc)}, [2]float32{bar.H / 2, 26}
	if !toMini {
		m.box[0], m.box[1] = m.box[1], m.box[0]
		m.disc[0], m.disc[1] = m.disc[1], m.disc[0]
		m.corner[0], m.corner[1] = m.corner[1], m.corner[0]
	}
	m.win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: w, Height: h, Content: ui.View(a.morphView)})
	m.win.Show()
	m.win.SetPosition(left, top)
	m.win.SetAlwaysOnTop(true)
	m.win.SetIgnoreMouseEvents(true)
	return true
}

// morphView draws the player on its way, and fades the main window as it
// goes from it or comes back to it.
func (a *app) morphView(c *ui.Context) {
	a.floatTheme(c)
	m := &a.morph
	if m.win == nil {
		return
	}
	now := c.Now()
	if m.at.IsZero() {
		m.at = now
	}
	t := clamp01(float32(now.Sub(m.at)) / float32(morphTime))
	// The window comes in over the capsule as that arrives, or goes off it
	// before that leaves: what is on it doesn't pop in, or out.
	const handOver = 0.62
	var move float32
	if m.toMini {
		move = t
		a.win.SetOpacity(float64(1 - easeRange(t, 0, 0.75)))
		if t >= handOver {
			if !m.handed {
				m.handed = true
				a.placeMini()
			}
			a.miniWin.SetOpacity(float64(easeRange(t, handOver, 1)))
		}
	} else {
		move = clamp01((t - 0.14) / 0.86)
		a.win.SetOpacity(float64(easeRange(t, 0.2, 1)))
		if !m.handed {
			a.miniWin.SetOpacity(float64(1 - easeRange(t, 0, 0.2)))
			if t >= 0.2 {
				m.handed = true
				a.miniWin.Hide()
				a.miniWin.SetOpacity(1)
			}
		}
	}
	k := 1 - (1-move)*(1-move)*(1-move)
	box, disc := lerpRect(m.box[0], m.box[1], k), lerpRect(m.disc[0], m.disc[1], k)
	ui.Box(c.Key("capsule")).Absolute().Left(box.X).Top(box.Y).Size(box.W, box.H).Radius(lerp(m.corner[0], m.corner[1], k)).
		Background(a.miniSurface()).Border(1, white.Alpha(0.14)).Shadow(0, 12, 30, -10, ui.RGBA(0, 0, 0, 0.45))
	if tr := a.pl.Current(); tr != nil {
		a.barDisc(c, tr, disc.W, discSmooth).Absolute().Left(disc.X).Top(disc.Y)
	}
	c.AnimationFrame()
	if t < 1 {
		return
	}
	if m.ending++; m.ending < 3 {
		return
	}
	win, toMini := m.win, m.toMini
	m.win = nil
	// Not from within its own frame.
	a.post(func() {
		win.Hide()
		win.Destroy()
		if toMini {
			a.win.Hide()
		}
		a.win.SetOpacity(1)
	})
}

// placeMini shows the mini window where it goes.
func (a *app) placeMini() {
	s := &a.settings
	if a.miniWin == nil {
		x, y := miniSpot(s.MiniX, s.MiniY)
		a.miniWin = a.floatWindow(" 迷你窗口", x, y, miniW, miniH, a.miniView, a.closeMini)
		a.miniWin.OnMove(func() {
			if a.miniWin.IsVisible() {
				b := a.miniWin.Bounds()
				s.MiniX, s.MiniY = b.X, b.Y
			}
		})
	}
	x, y := miniSpot(s.MiniX, s.MiniY)
	a.miniWin.SetOpacity(0)
	a.miniWin.Show()
	a.miniWin.SetPosition(x, y)
	a.miniWin.SetAlwaysOnTop(true)
	if a.morph.win == nil {
		a.miniWin.SetOpacity(1)
	}
}

// showMini puts the player in its small window, in place of the main one: the
// bar goes there, and the window around it fades.
func (a *app) showMini() {
	if a.win == nil || a.morph.win != nil {
		return
	}
	s := &a.settings
	x, y := miniSpot(s.MiniX, s.MiniY)
	if !a.startMorph(x, y, true) {
		a.placeMini()
		a.win.Hide()
	}
}

// closeMini goes back to the main window: the small one goes back into its bar.
func (a *app) closeMini() {
	if a.morph.win != nil {
		return
	}
	var x, y int
	if a.miniWin != nil {
		b := a.miniWin.Bounds()
		x, y = b.X, b.Y
	}
	if a.miniWin != nil && a.startMorph(x, y, false) {
		a.win.SetOpacity(0)
	} else if a.miniWin != nil {
		a.miniWin.Hide()
	}
	a.showMain()
}

// miniView is what plays, in small: over its cover's colors, as Now Playing,
// with how far it has played and how loud along with what controls it.
func (a *app) miniView(c *ui.Context) {
	a.floatTheme(c)
	a.followCover()
	pl := a.pl
	tr := pl.Current()
	root := ui.Box(c.Key("mini")).Fill().Radius(26).TextColor(white).
		Background(a.miniSurface()).Border(1, white.Alpha(0.14)).DragWindow()
	over := root.Hovered()
	root.Children(func() {
		ui.Row(c.Key("top")).Absolute().Top(12).Left(14).Right(12).Gap(13).AlignItems(ui.Start).Children(func() {
			if tr == nil {
				ui.Box(c).Size(miniDisc, miniDisc).Radius(999).Center().Background(white.Alpha(0.08)).Children(func() { mascotView(c, 52) })
			} else {
				a.barDisc(c, tr, miniDisc, discSmooth)
			}
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
						name, sub := "未在播放", ""
						if tr != nil {
							name, sub = tr.Name, firstArtist(tr)
						}
						ui.Text(c, name).FontSize(14).FontWeight(700).SingleLine()
						ui.Text(c, sub).FontSize(12).TextColor(white.Alpha(0.62)).SingleLine()
					})
					ink := white.Alpha(0.35)
					if over {
						ink = white.Alpha(0.85)
					}
					if iconButton(c.Key("back"), iconExpand, 14, ink, "回到主窗口").Size(26, 26).Clicked() {
						a.closeMini()
					}
				})
				ui.Row(c).Gap(2).AlignItems(ui.Center).Margin(3, 0, 0, -6).Children(func() {
					if iconButton(c.Key("prev"), iconPrev, 14, white, "上一首").Size(30, 28).Clicked() {
						pl.Prev()
					}
					if playButton(c.Key("play"), pl.Playing(), 18, white).Size(30, 28).Clicked() {
						pl.TogglePlay()
					}
					if iconButton(c.Key("next"), iconNext, 14, white, "下一首").Size(30, 28).Clicked() {
						pl.Next()
					}
					ui.Spacer(c)
					a.volumeSlider(c, white.Alpha(0.75), white.Alpha(0.18), 92)
				})
			})
		})
		a.scrubber(c.Key("mini-scrub"), &a.miniScrub, white.Alpha(0.18), white.Alpha(0.85), 14, ui.Color{}).
			Absolute().Left(18).Right(18).Bottom(5)
	})
	if pl.Playing() {
		c.After(time.Second)
	}
}
