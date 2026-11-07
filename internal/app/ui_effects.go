package app

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/player"
)

// The sound effects, as a pane of their own from a song's details: a
// preset to start from, ten bands to shape, the bass and the room. They
// shape every song, and are kept.

// effectsView is the pane: open or not, and whether what was changed has
// yet to be heard at once and kept.
type effectsView struct {
	open  bool
	dirty bool
	held  bool // a slider is being dragged, this frame
}

// effectPreset is a starting point: the bands, the bass and the width.
type effectPreset struct {
	id, label string
	bands     [10]float64
	bass      float64
	width     float64
}

// effectPresets are the presets the pane offers, the first leaving the
// sound as it is.
var effectPresets = []effectPreset{
	{"flat", "原声", [10]float64{}, 0, 0},
	{"pop", "流行", [10]float64{-1, 0, 1.5, 3, 3.5, 2.5, 1, 0, 1, 1.5}, 0.15, 0.1},
	{"rock", "摇滚", [10]float64{4, 3, 2, 0, -1.5, -1, 1, 2.5, 3.5, 4}, 0.2, 0.15},
	{"jazz", "爵士", [10]float64{3, 2, 1, 2, -1, -1, 0, 1, 2, 3}, 0, 0.2},
	{"classical", "古典", [10]float64{4, 3, 2, 1, -1, -1, 0, 2, 3, 4}, 0, 0.3},
	{"electronic", "电子", [10]float64{4, 3.5, 1, 0, -2, 1.5, 1, 1, 3, 4}, 0.35, 0.25},
	{"vocal", "人声", [10]float64{-2, -2, -1, 1, 3, 4, 3.5, 1.5, 0, -1}, 0, 0},
	{"bass", "重低音", [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}, 0.5, 0},
	{"night", "夜间", [10]float64{0, 0, 0, 0, 0, 0, -1, -2, -3.5, -5}, 0, 0},
}

// customPreset is what the bands are once the listener moved one.
const customPreset = "custom"

func (a *app) openEffects() {
	a.effects.open = true
	a.redraw()
}

// effectsChanged has what plays shaped as the settings say: heard at once
// once no slider is held, rather than after the second decoded ahead, and
// kept then.
func (a *app) effectsChanged() {
	a.pl.SetEffects(a.settings.Effects, false)
	a.effects.dirty = true
}

// effectsSettle hears and keeps what changed, once nothing is held.
func (a *app) effectsSettle() {
	if !a.effects.dirty || a.effects.held {
		return
	}
	a.effects.dirty = false
	a.pl.SetEffects(a.settings.Effects, true)
	a.settings.save()
}

// usePreset starts from p, turning the effects on.
func (a *app) usePreset(p effectPreset) {
	e := &a.settings.Effects
	e.On, e.Bands, e.Bass, e.Width = true, p.bands, p.bass, p.width
	a.settings.EffectsPreset = p.id
	a.effectsChanged()
}

// presetLabel names the preset the effects are at.
func (a *app) presetLabel() string {
	for _, p := range effectPresets {
		if p.id == a.settings.EffectsPreset {
			return p.label
		}
	}
	if a.settings.EffectsPreset == customPreset {
		return "自定义"
	}
	return effectPresets[0].label
}

// effectsDialog is the pane of the effects.
func (a *app) effectsDialog(c *ui.Context) {
	v := &a.effects
	if !v.open {
		return
	}
	ink, pal := a.inkOn(false), a.pal
	e := &a.settings.Effects
	w, h := c.Size()
	v.held = false
	a.dialog(c, "effects", 0.35, &v.open, func(panel ui.Element) {
		panel.Width(min(700, w-80)).MaxHeight(h - 56).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			ui.Scroll(c.Key("effects-scroll")).Shrink(1).Padding(24).Gap(18).Children(func() {
				ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, "音效").FontSize(20).FontWeight(700)
						sub := "均衡器、低音和空间感，对所有歌曲生效"
						if e.On {
							sub = "当前：" + a.presetLabel()
						}
						ui.Text(c, sub).FontSize(12.5).TextColor(ink.muted)
					})
					if ui.Switch(c.Key("effects-on"), &e.On).Label("开启音效").Changed() {
						a.effectsChanged()
					}
					if iconButton(c.Key("effects-close"), iconX, 16, ink.muted, "关闭").Clicked() {
						v.open = false
					}
				})

				// Off, what would be shaped stays in view, faded: a touch
				// turns the effects on.
				body := ui.Column(c).Gap(18)
				if !e.On {
					body.Opacity(0.55)
				}
				body.Children(func() {
					ui.Row(c).Wrap().GapX(8).GapY(8).Children(func() {
						for _, p := range effectPresets {
							chosen := e.On && a.settings.EffectsPreset == p.id
							bg, fg := ink.text.Alpha(0.08), ink.text
							if chosen {
								bg, fg = pal.accent, pal.onAccent
							}
							if pillButton(c.Key("preset-"+p.id), nil, p.label, bg, fg).Height(32).PaddingX(12).Clicked() {
								a.usePreset(p)
							}
						}
						// What the bands are once moved by hand: it shows where they
						// are, and is chosen by moving one.
						bg, fg := ink.text.Alpha(0.08), ink.muted
						if a.settings.EffectsPreset == customPreset && e.On {
							bg, fg = pal.accent, pal.onAccent
						}
						pillButton(c.Key("preset-custom"), nil, "自定义", bg, fg).Height(32).PaddingX(12)
					})
					a.equalizer(c, ink)
					ui.Column(c).Radius(14).Background(ink.text.Alpha(0.05)).Padding(4, 16).Children(func() {
						a.effectSlider(c, "低音增强", "加强 100Hz 以下的低频", &e.Bass, ink)
						a.effectSlider(c, "空间环绕", "加宽左右声道，声音更开阔", &e.Width, ink)
					})
				})

				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					if pillButton(c.Key("effects-reset"), iconReload, "恢复原声", ink.text.Alpha(0.08), ink.text).Clicked() {
						a.usePreset(effectPresets[0])
					}
					ui.Spacer(c)
					ui.Text(c, "调整后约半秒内生效").FontSize(11.5).TextColor(ink.faint)
					if pillButton(c.Key("effects-done"), iconCheck, "完成", pal.accent, pal.onAccent).Clicked() {
						v.open = false
					}
				})
			})
		})
	})
	a.effectsSettle()
}

// eqHeight is how tall the equalizer's sliders run.
const eqHeight = 150

// equalizer is the ten bands, a slider standing up each, over the curve
// they make together.
func (a *app) equalizer(c *ui.Context, ink paneInk) {
	e := &a.settings.Effects
	accent := a.pal.accent
	box := ui.Column(c.Key("eq")).Radius(14).Background(ink.text.Alpha(0.05)).Padding(14, 12, 10, 12).Gap(6)
	box.Children(func() {
		row := ui.Row(c.Key("eq-bands")).AlignItems(ui.Stretch)
		bands := e.Bands
		row.Draw(func(p *ui.Painter, r ui.Rect) {
			// The zero line, and the curve the bands make, filled to it.
			top := r.Y + 20 + knobEQ/2
			span := float32(eqHeight - knobEQ)
			yOf := func(g float64) float32 { return top + span*float32(1-(g+player.EffectRange)/(2*player.EffectRange)) }
			zero := yOf(0)
			p.Fill(ui.Rect{X: r.X + 6, Y: zero - 0.5, W: r.W - 12, H: 1}, ink.text.Alpha(0.12), 0)
			col := r.W / float32(len(bands))
			pts := make([]ui.Point, len(bands))
			for i, g := range bands {
				pts[i] = ui.Point{X: r.X + col*(float32(i)+0.5), Y: yOf(g)}
			}
			curve := (&ui.Path{}).MoveTo(pts[0].X, pts[0].Y)
			for i := 0; i+1 < len(pts); i++ {
				// Through each point, as smooth as Catmull-Rom makes it.
				p0, p1, p2, p3 := pts[max(i-1, 0)], pts[i], pts[i+1], pts[min(i+2, len(pts)-1)]
				curve.CubeTo(p1.X+(p2.X-p0.X)/6, p1.Y+(p2.Y-p0.Y)/6, p2.X-(p3.X-p1.X)/6, p2.Y-(p3.Y-p1.Y)/6, p2.X, p2.Y)
			}
			fill := (&ui.Path{}).MoveTo(pts[0].X, zero)
			fill.LineTo(pts[0].X, pts[0].Y)
			for i := 0; i+1 < len(pts); i++ {
				p0, p1, p2, p3 := pts[max(i-1, 0)], pts[i], pts[i+1], pts[min(i+2, len(pts)-1)]
				fill.CubeTo(p1.X+(p2.X-p0.X)/6, p1.Y+(p2.Y-p0.Y)/6, p2.X-(p3.X-p1.X)/6, p2.Y-(p3.Y-p1.Y)/6, p2.X, p2.Y)
			}
			fill.LineTo(pts[len(pts)-1].X, zero).Close()
			p.FillPath(fill, accent.Alpha(0.14))
			p.StrokePath(curve, 2, accent.Alpha(0.85))
		})
		row.Children(func() {
			for i := range e.Bands {
				ui.Column(c.Key(i)).Grow(1).Basis(0).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Text(c, gainLabel(e.Bands[i])).FontSize(10.5).FontFeatures("tnum").TextColor(ink.muted).Height(16)
					a.bandSlider(c, i, ink)
					ui.Text(c, freqLabel(player.EffectBands[i])).FontSize(11).FontFeatures("tnum").TextColor(ink.muted)
				})
			}
		})
	})
}

// knobEQ is the size of a band slider's knob.
const knobEQ = 16

// bandSlider is band i, standing up: up lifts it, down cuts it.
func (a *app) bandSlider(c *ui.Context, i int, ink paneInk) {
	e := &a.settings.Effects
	label := freqLabel(player.EffectBands[i]) + "Hz"
	s := ui.SliderBase(c.Key("band"), &e.Bands[i], -player.EffectRange, player.EffectRange).Vertical().
		Size(knobEQ+12, eqHeight).PaddingY(knobEQ / 2).Step(0.5).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
	if s.Changed() {
		e.On = true
		a.settings.EffectsPreset = customPreset
		a.effectsChanged()
	}
	if s.Pressed() {
		a.effects.held = true
	}
	if s.DoubleClicked() {
		// Back to nothing, as a fader is.
		e.Bands[i] = 0
		a.settings.EffectsPreset = customPreset
		a.effectsChanged()
	}
	g := float32((e.Bands[i] + player.EffectRange) / (2 * player.EffectRange))
	hot := s.Hovered() || s.Pressed()
	accent := a.pal.accent
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		top, foot := r.Y+knobEQ/2, r.Y+r.H-knobEQ/2
		x := r.X + (r.W-3)/2
		p.Fill(ui.Rect{X: x, Y: top, W: 3, H: foot - top}, ink.text.Alpha(0.12), 1.5)
		y := foot - (foot-top)*g
		mid := (top + foot) / 2
		p.Fill(ui.Rect{X: x, Y: min(y, mid), W: 3, H: max(y, mid) - min(y, mid)}, accent, 1.5)
		size := float32(knobEQ)
		if hot {
			size += 2
		}
		k := ui.Rect{X: r.X + (r.W-size)/2, Y: y - size/2, W: size, H: size}
		p.Shadow(k, size/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.3))
		p.Fill(k, white, size/2)
	})
}

// effectSlider is a row of the pane: what it does, and how much of it.
func (a *app) effectSlider(c *ui.Context, label, desc string, v *float64, ink paneInk) {
	ui.Row(c.Key(label)).Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(13.5).FontWeight(500)
			ui.Text(c, desc).FontSize(12).TextColor(ink.muted)
		})
		ui.Text(c, strconv.Itoa(int(*v*100+0.5))+"%").FontSize(12).FontFeatures("tnum").TextColor(ink.muted).Width(40).TextAlign(ui.End)
		s := ui.SliderBase(c.Key("slider"), v, 0, 1).Width(200).Height(22).Step(0.01).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
		if s.Changed() {
			a.settings.Effects.On = true
			a.effectsChanged()
		}
		if s.Pressed() {
			a.effects.held = true
		}
		at, accent := float32(*v), a.pal.accent
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			const thick, knob = 4, 16
			y := r.Y + (r.H-thick)/2
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, ink.text.Alpha(0.12), thick/2)
			p.Fill(ui.Rect{X: r.X, Y: y, W: max(thick, r.W*at), H: thick}, accent, thick/2)
			k := ui.Rect{X: r.X + (r.W-knob)*at, Y: r.Y + (r.H-knob)/2, W: knob, H: knob}
			p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.3))
			p.Fill(k, white, knob/2)
		})
	})
}

// gainLabel says a band's gain: +3, -2.5, 0.
func gainLabel(g float64) string {
	switch {
	case g == 0:
		return "0"
	case g == float64(int(g)):
		return fmt.Sprintf("%+d", int(g))
	}
	return fmt.Sprintf("%+.1f", g)
}

// freqLabel says a band's frequency: 31, 1k, 16k.
func freqLabel(f float64) string {
	if f >= 1000 {
		return strconv.Itoa(int(f/1000)) + "k"
	}
	return strconv.Itoa(int(f))
}
// effectsNote is what the menu says of the effects beside their name.
func (a *app) effectsNote() string {
	if !a.settings.Effects.On {
		return "关闭"
	}
	return a.presetLabel()
}
