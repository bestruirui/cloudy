package app

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/player"
)

// The sound effects, as a pane of their own from a song's details: a
// preset to start from, ten bands to shape, the bass and the room. They
// shape every song, and are kept.

// effectsView is the pane: open or not, which of its pages shows, and
// whether what was changed has yet to be heard at once and kept.
type effectsView struct {
	open  bool
	page  int // 0 the presets, 1 what the listener shapes
	dirty bool
	held  bool // a slider is being dragged, this frame
}

// effectPreset is a starting point: the bands, and how much of the bass,
// the room, the voice and the evening out; and how its card looks, its
// colors and what it shows.
type effectPreset struct {
	id, label, desc string
	bands           [10]float64
	bass, width     float64
	vocal, loud     float64
	from, to        string // the card's colors, top left to bottom right
	icon            *ui.SVG
}

// effectPresets are the presets the pane offers, the first leaving the
// sound as it is.
var effectPresets = []effectPreset{
	{"flat", "原声", "不做处理，保留录音原貌", [10]float64{}, 0, 0, 0, 0, "#7b8494", "#3b4252", iconWave},
	{"pop", "流行", "人声靠前，节奏明快", [10]float64{-1, 0, 1.5, 3, 3.5, 2.5, 1, 0, 1, 1.5}, 0.15, 0.1, 0.2, 0, "#f472b6", "#f97316", iconParty},
	{"rock", "摇滚", "鼓点有力，吉他明亮", [10]float64{4, 3, 2, 0, -1.5, -1, 1, 2.5, 3.5, 4}, 0.2, 0.15, 0, 0, "#ef4444", "#5b1414", iconGuitar},
	{"jazz", "爵士", "温暖松弛，像在小酒馆", [10]float64{3, 2, 1, 2, -1, -1, 0, 1, 2, 3}, 0, 0.2, 0, 0, "#6366f1", "#1e1b4b", iconWine},
	{"classical", "古典", "层次开阔，弦乐通透", [10]float64{4, 3, 2, 1, -1, -1, 0, 2, 3, 4}, 0, 0.3, 0, 0, "#d4a373", "#6b4a2b", iconPiano},
	{"electronic", "电子", "低频饱满，高频闪亮", [10]float64{4, 3.5, 1, 0, -2, 1.5, 1, 1, 3, 4}, 0.35, 0.25, 0, 0, "#22d3ee", "#6d28d9", iconZap},
	{"vocal", "人声", "突出歌声，听清每个字", [10]float64{-2, -2, -1, 1, 3, 4, 3.5, 1.5, 0, -1}, 0, 0, 0.6, 0, "#fb7185", "#881337", iconMicVocal},
	{"bass", "重低音", "低频下潜，鼓点更震撼", [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}, 0.5, 0, 0, 0, "#f59e0b", "#5c2a07", iconDisc},
	{"night", "夜间", "高音柔和，音量平稳不吵人", [10]float64{0, 0, 0, 0, 0, 0, -1, -2, -3.5, -5}, 0, 0, 0, 0.6, "#3b82f6", "#0b1530", iconMoonStar},
}

// customPreset is what the effects are once the listener shaped them.
const customPreset = "custom"

func (a *app) openEffects() {
	a.effects.open = true
	if a.settings.EffectsPreset == customPreset {
		a.effects.page = 1
	}
	a.redraw()
}

// effectsChanged has what plays shaped as the settings say: heard at once
// once no slider is held, rather than after the second decoded ahead, and
// kept then.
func (a *app) effectsChanged() {
	a.pl.SetEffects(a.currentEffects(), false)
	a.effects.dirty = true
}

// effectsSettle hears and keeps what changed, once nothing is held.
func (a *app) effectsSettle() {
	if !a.effects.dirty || a.effects.held {
		return
	}
	a.effects.dirty = false
	a.pl.SetEffects(a.currentEffects(), true)
	a.settings.save()
}

// usePreset starts from p, turning the effects on.
func (a *app) usePreset(p effectPreset) {
	e := &a.settings.Effects
	e.On, e.Bands, e.Bass, e.Width, e.Vocal, e.Loudness = true, p.bands, p.bass, p.width, p.vocal, p.loud
	a.settings.EffectsPreset = p.id
	a.effectsChanged()
}

// shaped marks the effects the listener's own, turned on, as a control
// of the custom page moved.
func (a *app) shaped() {
	a.settings.Effects.On = true
	a.settings.EffectsPreset = customPreset
	a.effectsChanged()
}

// presetLabel names the preset the effects are at.
func (a *app) presetLabel() string {
	for _, p := range effectPresets {
		if p.id == a.settings.EffectsPreset {
			return p.label
		}
	}
	switch a.settings.EffectsPreset {
	case customPreset:
		return "自定义"
	case smartPreset:
		return "智能音效"
	case moodPreset:
		return "情绪调音"
	}
	return effectPresets[0].label
}

// effectsDialog is the pane of the effects: the presets as cards, or the
// controls to shape the sound by hand.
func (a *app) effectsDialog(c *ui.Context) {
	v := &a.effects
	if !v.open {
		return
	}
	ink, pal := a.inkOn(false), a.pal
	e := &a.settings.Effects
	w, h := c.Size()
	pw := min(720, w-80)
	v.held = false
	a.dialog(c, "effects", 0.35, &v.open, func(panel ui.Element) {
		panel.Width(pw).MaxHeight(h - 56).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			ui.Scroll(c.Key("effects-scroll")).Shrink(1).Padding(24).Gap(18).Children(func() {
				ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, "音效").FontSize(20).FontWeight(700)
						sub := "已关闭，选择一个预设或手动调节即可开启"
						if e.On {
							sub = "当前：" + a.presetLabel() + "，对所有歌曲生效"
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
				ui.Row(c).Children(func() {
					a.segmented(c.Key("effects-page"), &v.page, "预设", "自定义")
				})
				if v.page == 0 {
					a.presetCards(c, (pw-48-2*12)/3)
				} else {
					body := ui.Column(c).Gap(18)
					if !e.On {
						body.Opacity(0.55)
					}
					body.Children(func() {
						a.equalizer(c, ink)
						a.enhanceCard(c, ink)
						ui.Column(c).Radius(14).Background(ink.text.Alpha(0.05)).Padding(4, 16).Children(func() {
							a.effectSlider(c, "人声增强", "让居中的人声更靠前、更清楚，伴奏不受影响", &e.Vocal, ink)
							a.effectSlider(c, "响度均衡", "压低过响的段落、抬起轻柔的段落，音量更平稳", &e.Loudness, ink)
							a.roomRow(c, ink)
						})
					})
				}
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

// presetCards is the presets' page: effects that choose themselves for
// each song, the sound placed round the head, and the presets of a kind
// of music, as cards.
func (a *app) presetCards(c *ui.Context, colw float32) {
	ink := a.inkOn(false)
	e := &a.settings.Effects
	section := func(title, note string) {
		ui.Row(c).Gap(8).AlignItems(ui.End).Margin(4, 0, 0, 0).Children(func() {
			ui.Text(c, title).FontSize(13.5).FontWeight(600)
			ui.Text(c, note).FontSize(11.5).TextColor(ink.faint)
		})
	}
	ui.Column(c).Gap(12).Children(func() {
		section("智能调音", "每首歌自动选择，切歌时平滑过渡")
		ui.Grid(c).Columns(2).GapX(12).GapY(12).Children(func() {
			for _, m := range []struct {
				id, label, desc, from, to string
				icon                      *ui.SVG
			}{
				{smartPreset, "智能音效", "按网易云的曲风自动匹配最合适的预设", "#2563eb", "#7c3aed", iconWand},
				{moodPreset, "情绪调音", "读取歌曲的情绪标签和歌词，按情绪调音", "#db2777", "#7c2d92", iconHeart},
			} {
				chosen := e.On && a.settings.EffectsPreset == m.id
				desc := m.desc
				if chosen && a.auto.note != "" {
					desc = a.auto.note
				}
				if a.themedCard(c, m.id, m.label, desc, m.from, m.to, m.icon, chosen, 104) {
					a.useAuto(m.id)
				}
			}
		})
		section("空间音效", "戴耳机效果最好，可以和任意预设一起使用")
		ui.Grid(c).Columns(2).GapX(12).GapY(12).Children(func() {
			for _, m := range []struct {
				id, label, desc, from, to string
				icon                      *ui.SVG
			}{
				{player.Spatial8D, "8D 环绕", "声音绕着头部缓缓转圈", "#0891b2", "#1e3a8a", iconFan},
				{player.Spatial360, "360° 环绕", "像坐在两只音箱前，声音在头外", "#059669", "#134e4a", iconGlobe},
			} {
				chosen := e.On && e.Spatial == m.id
				if a.themedCard(c, "spatial-"+m.id, m.label, m.desc, m.from, m.to, m.icon, chosen, 104) {
					if e.Spatial == m.id {
						e.Spatial = ""
					} else {
						e.Spatial, e.On = m.id, true
					}
					a.effectsChanged()
				}
			}
		})
		if e.On && e.Spatial == player.Spatial8D {
			ui.Column(c).Radius(14).Background(ink.text.Alpha(0.05)).Padding(0, 16).Children(func() {
				period := 16 - 12*e.Orbit
				a.effectSlider(c, "转速", fmt.Sprintf("约 %.0f 秒转一圈", period), &e.Orbit, ink)
			})
		}
		section("风格预设", "")
		ui.Grid(c).Columns(3).GapX(12).GapY(12).Children(func() {
			for i := range effectPresets {
				p := &effectPresets[i]
				chosen := e.On && a.settings.EffectsPreset == p.id
				if a.themedCard(c, "preset-"+p.id, p.label, p.desc, p.from, p.to, p.icon, chosen, 116) {
					a.usePreset(*p)
				}
			}
		})
	})
}

// useAuto has the effects choose themselves for each song, by id: by
// genre, or by mood.
func (a *app) useAuto(id string) {
	a.settings.Effects.On = true
	a.settings.EffectsPreset = id
	a.auto.song, a.auto.fx, a.auto.note = 0, nil, ""
	a.autoFollow()
	a.effectsChanged()
}

// themedCard is a card of the effects: its colors, its theme drawn large
// and faint off the corner, what it is, and a check while it is chosen.
// It reports a click.
func (a *app) themedCard(c *ui.Context, key, label, desc, from, to string, icon *ui.SVG, chosen bool, height float32) bool {
	card := ui.ButtonBase(c.Key(key)).Column().AlignItems(ui.Start).Justify(ui.Start).Height(height).Radius(14).Padding(14, 14).Gap(4).
		FocusRing(false).Label(label).Clip()
	card.LinearGradient(ui.LinearGradient{From: ui.Hex(from), To: ui.Hex(to), Angle: 135, Oklab: true})
	edge := ui.Transparent
	if chosen {
		edge = white
	}
	card.Border(2, card.AnimateColor("edge", edge, ui.Animation{Duration: 140 * time.Millisecond}))
	pose := ui.Pose{}
	if card.Hovered() {
		pose.Y = -2
	}
	if card.Pressed() {
		pose = ui.Pose{Scale: 0.97}
	}
	card.AnimatePose(pose, ui.Snappy)
	if card.Hovered() || chosen {
		card.Shadow(0, 10, 24, -8, ui.Hex(to).Alpha(0.7))
	}
	card.Children(func() {
		ui.Icon(c, icon).Absolute().Right(-10).Top(-6).Size(78, 78).TextColor(white.Alpha(0.16))
		ui.Text(c, label).FontSize(16).FontWeight(700).TextColor(white)
		ui.Text(c, desc).FontSize(11.5).TextColor(white.Alpha(0.85)).MaxLines(2).LineHeight(1.35).Shrink(1).MinWidth(0).Margin(0, 26, 0, 0)
		if chosen {
			ui.Box(c).Absolute().Top(10).Right(10).Size(20, 20).Radius(999).Background(white).Center().Children(func() {
				ui.Icon(c, iconCheck).Size(12, 12).TextColor(ui.Hex(to))
			})
		}
	})
	return card.Clicked()
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
		a.shaped()
	}
	if s.Pressed() {
		a.effects.held = true
	}
	if s.DoubleClicked() {
		// Back to nothing, as a fader is.
		e.Bands[i] = 0
		a.shaped()
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
			a.shaped()
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

// enhanceCard is the bass and the highs, each down or up, and the room's
// depth and width, two a row, with what puts them back.
func (a *app) enhanceCard(c *ui.Context, ink paneInk) {
	e := &a.settings.Effects
	ui.Column(c.Key("enhance")).Radius(14).Background(ink.text.Alpha(0.05)).Padding(12, 16, 16, 16).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "增强音效").FontSize(13.5).FontWeight(600).Grow(1)
			if pillButton(c.Key("enhance-reset"), nil, "重置", ink.text.Alpha(0.08), ink.text).Height(28).PaddingX(14).Clicked() {
				e.Bass, e.Treble, e.Depth, e.Width = 0, 0, 0, 0
				a.shaped()
			}
		})
		ui.Grid(c).Columns(2).GapX(28).GapY(14).Children(func() {
			a.compactSlider(c, "低音", &e.Bass, -1, ink)
			a.compactSlider(c, "高音", &e.Treble, -1, ink)
			a.compactSlider(c, "环绕深度", &e.Depth, 0, ink)
			a.compactSlider(c, "环绕强度", &e.Width, 0, ink)
		})
	})
}

// compactSlider is one of the enhancements: − and + at its ends, its name
// and how much under it. From lo, -1 or 0, to 1: one that goes down and up
// fills from its middle.
func (a *app) compactSlider(c *ui.Context, label string, v *float64, lo float64, ink paneInk) {
	ui.Column(c.Key(label)).Gap(4).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "−").FontSize(13).TextColor(ink.muted).Width(10)
			s := ui.SliderBase(c.Key("slider"), v, lo, 1).Grow(1).Height(22).Step(0.02).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
			if s.Changed() {
				a.shaped()
			}
			if s.Pressed() {
				a.effects.held = true
			}
			if s.DoubleClicked() {
				*v = 0
				a.shaped()
			}
			at, from, accent := float32((*v-lo)/(1-lo)), float32(-lo/(1-lo)), a.pal.accent
			s.Draw(func(p *ui.Painter, r ui.Rect) {
				const thick, knob = 4, 16
				y := r.Y + (r.H-thick)/2
				span := r.W - knob
				x := func(f float32) float32 { return r.X + knob/2 + span*f }
				p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, ink.text.Alpha(0.12), thick/2)
				a0, a1 := min(x(from), x(at)), max(x(from), x(at))
				p.Fill(ui.Rect{X: a0, Y: y, W: max(a1-a0, 0.1), H: thick}, accent, thick/2)
				k := ui.Rect{X: x(at) - knob/2, Y: r.Y + (r.H-knob)/2, W: knob, H: knob}
				p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.3))
				p.Fill(k, white, knob/2)
			})
			ui.Text(c, "+").FontSize(13).TextColor(ink.muted).Width(10).TextAlign(ui.End)
		})
		amount := strconv.Itoa(int(math.Round(*v*100))) + "%"
		if lo < 0 && *v > 0 {
			amount = "+" + amount
		}
		ui.Text(c, label+"  "+amount).FontSize(12).TextColor(ink.muted).TextAlign(ui.Center).FontFeatures("tnum")
	})
}

// roomRow chooses the room the sound plays in.
func (a *app) roomRow(c *ui.Context, ink paneInk) {
	e := &a.settings.Effects
	ui.Row(c.Key("room")).Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, "环境音效").FontSize(13.5).FontWeight(500)
			ui.Text(c, "让声音像在不同的空间里播放").FontSize(12).TextColor(ink.muted)
		})
		labels := make([]string, len(player.Rooms))
		chosen := player.Rooms[0].Label
		for i, r := range player.Rooms {
			labels[i] = r.Label
			if r.ID == e.Room {
				chosen = r.Label
			}
		}
		if a.paneSelect(c.Key("room-select"), "环境音效", &chosen, labels) {
			for _, r := range player.Rooms {
				if r.Label == chosen {
					e.Room = r.ID
				}
			}
			a.shaped()
		}
	})
}
