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

// effectsDialog is the pane of the effects: the presets as tiles, or the
// controls to shape the sound by hand, in the settings' cards and rows.
func (a *app) effectsDialog(c *ui.Context) {
	v := &a.effects
	if !v.open {
		return
	}
	ink, pal := a.inkOn(false), a.pal
	e := &a.settings.Effects
	w, h := c.Size()
	pw := min(760, w-80)
	v.held = false
	a.dialog(c, "effects", 0.35, &v.open, func(panel ui.Element) {
		panel.Width(pw).MaxHeight(h - 56).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			dialogHeader(c, ink, "音效", "", func() { v.open = false })
			k := &settingsKit{a: a, c: c, ink: ink}
			ui.Scroll(c.Key("effects-scroll")).Shrink(1).Padding(0, 28, 8, 28).Children(func() {
				k.card(func() {
					sub := "已关闭；选一个预设或手动调节就会打开"
					if e.On {
						sub = "当前：" + a.presetLabel() + "，对所有歌曲生效"
					}
					k.row("开启音效", sub, func() {
						if ui.Switch(c.Key("effects-on"), &e.On).Label("开启音效").Changed() {
							a.effectsChanged()
						}
					})
				})
				ui.Row(c).Margin(18, 0, 0, 0).Children(func() {
					a.segmented(c.Key("effects-page"), &v.page, "预设", "自定义")
				})
				body := ui.Column(c)
				if !e.On && v.page == 1 {
					body.Opacity(0.55)
				}
				body.Children(func() {
					if v.page == 0 {
						a.presetTiles(k, (pw-56-2*10)/3)
					} else {
						a.customEffects(k)
					}
				})
			})
			dialogFooter(c, func() {
				if settingButton(c.Key("effects-reset"), ink, "恢复原声", iconReload, false, false).Clicked() {
					a.usePreset(effectPresets[0])
				}
				ui.Spacer(c)
				ui.Text(c, "调整后约半秒内生效").FontSize(11.5).TextColor(ink.faint)
				if primaryButton(c.Key("effects-done"), "完成", iconCheck, pal, false).Clicked() {
					v.open = false
				}
			})
		})
	})
	a.effectsSettle()
}

// presetTiles is the presets' page: effects that choose themselves for
// each song, the sound placed round the head, and the presets of a kind
// of music, as tiles, each group saying which of it is chosen.
func (a *app) presetTiles(k *settingsKit, colw float32) {
	c, ink := k.c, k.ink
	e := &a.settings.Effects
	section := func(title, chosen, note string) {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(18, 0, 8, 18).Children(func() {
			ui.Text(c, title).FontSize(12.5).FontWeight(600).TextColor(ink.muted)
			if chosen != "" {
				ui.Text(c, "· "+chosen).FontSize(12.5).FontWeight(600).TextColor(a.pal.accent)
			}
			if note != "" {
				ui.Text(c, note).FontSize(11.5).TextColor(ink.faint).Margin(0, 0, 0, 6)
			}
		})
	}
	auto := []struct {
		id, label, desc, color string
		icon                   *ui.SVG
	}{
		{smartPreset, "智能音效", "按网易云的曲风自动匹配最合适的预设", "#6d6af0", iconWand},
		{moodPreset, "情绪调音", "读取歌曲的情绪标签和歌词，按情绪调音", "#e0559a", iconHeart},
	}
	chosenAuto := ""
	for _, m := range auto {
		if e.On && a.settings.EffectsPreset == m.id {
			chosenAuto = m.label
		}
	}
	section("智能调音", chosenAuto, "每首歌自动选择，切歌时平滑过渡")
	ui.Grid(c).Columns(2).GapX(10).GapY(10).Children(func() {
		for _, m := range auto {
			chosen := e.On && a.settings.EffectsPreset == m.id
			desc := m.desc
			if chosen && a.auto.note != "" {
				desc = a.auto.note
			}
			if a.presetTile(c, m.id, m.label, desc, m.color, m.icon, chosen) {
				a.useAuto(m.id)
			}
		}
	})
	spatial := []struct {
		id, label, desc, color string
		icon                   *ui.SVG
	}{
		{player.Spatial8D, "8D 环绕", "声音绕着头部缓缓转圈", "#1aa3c2", iconFan},
		{player.Spatial360, "360° 环绕", "像坐在两只音箱前，声音在头外", "#21a37a", iconGlobe},
	}
	chosenSpatial := ""
	for _, m := range spatial {
		if e.On && e.Spatial == m.id {
			chosenSpatial = m.label
		}
	}
	section("空间音效", chosenSpatial, "戴耳机效果最好，可以和任意预设一起用")
	ui.Grid(c).Columns(2).GapX(10).GapY(10).Children(func() {
		for _, m := range spatial {
			if a.presetTile(c, "spatial-"+m.id, m.label, m.desc, m.color, m.icon, e.On && e.Spatial == m.id) {
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
		ui.Box(c).Height(10)
		k.card(func() {
			period := 16 - 12*e.Orbit
			k.row("转速", fmt.Sprintf("约 %.0f 秒转一圈", period), func() { a.effectSlider(c, "转速", &e.Orbit, 0, ink) })
		})
	}
	chosenStyle := ""
	for _, p := range effectPresets {
		if e.On && a.settings.EffectsPreset == p.id {
			chosenStyle = p.label
		}
	}
	section("风格预设", chosenStyle, "")
	ui.Grid(c).Columns(3).GapX(10).GapY(10).Children(func() {
		for i := range effectPresets {
			p := &effectPresets[i]
			if a.presetTile(c, "preset-"+p.id, p.label, p.desc, p.from, p.icon, e.On && a.settings.EffectsPreset == p.id) {
				a.usePreset(*p)
			}
		}
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

// presetTile is a tile of the effects, on the pane's glass as the
// settings' cards are: its theme in its color on a small square, what it
// is, and an edge and check in the accent while it is chosen. It reports a
// click.
func (a *app) presetTile(c *ui.Context, key, label, desc, color string, icon *ui.SVG, chosen bool) bool {
	ink := a.inkOn(false)
	fill, edge := a.cardColors()
	tint := ui.Hex(color)
	if a.pal.dark {
		tint = tint.Mix(white, 0.18)
	}
	tile := ui.ButtonBase(c.Key(key)).Row().AlignItems(ui.Center).Height(72).Radius(14).Padding(12, 14).Gap(12).
		FocusRing(false).Label(label)
	bg, line, width := fill, edge, float32(0.5)
	if tile.Hovered() {
		bg = fill.Mix(ink.text, 0.04)
	}
	if chosen {
		line, width = a.pal.accent, 1.5
	}
	tile.Background(tile.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).
		Border(width, tile.AnimateColor("edge", line, ui.Animation{Duration: 140 * time.Millisecond}))
	pose := ui.Pose{}
	if tile.Pressed() {
		pose.Scale = 0.98
	}
	tile.AnimatePose(pose, ui.Snappy)
	tile.Children(func() {
		ui.Box(c).Size(38, 38).Radius(8).Shrink(0).Center().Background(tint.Alpha(0.16)).Children(func() {
			ui.Icon(c, icon).Size(20, 20).TextColor(tint)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, label).FontSize(14).FontWeight(600).SingleLine()
			ui.Text(c, desc).FontSize(11.5).TextColor(ink.muted).MaxLines(2).LineHeight(1.35)
		})
		if chosen {
			ui.Box(c).Size(18, 18).Radius(999).Shrink(0).Background(a.pal.accent).Center().Children(func() {
				ui.Icon(c, iconCheck).Size(11, 11).TextColor(a.pal.onAccent)
			})
		}
	})
	return tile.Clicked()
}

// customEffects is the page of what the listener shapes: the bands, the
// enhancements, the voice and the evening out, and the room, in the
// settings' groups.
func (a *app) customEffects(k *settingsKit) {
	c, ink := k.c, k.ink
	e := &a.settings.Effects
	slider := func(label, desc string, v *float64, lo float64) {
		k.row(label, desc, func() { a.effectSlider(c, label, v, lo, ink) })
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(18, 0, 8, 18).Children(func() {
		ui.Text(c, "均衡器").FontSize(12.5).FontWeight(600).TextColor(ink.muted).Grow(1)
		ui.Text(c, "双击推子归零").FontSize(11.5).TextColor(ink.faint)
	})
	k.card(func() { a.equalizer(c, ink) })
	k.group("增强", func() {
		slider("低音", "向左减弱，向右加强", &e.Bass, -1)
		slider("高音", "向左减弱，向右加强", &e.Treble, -1)
		slider("环绕深度", "声音前后的层次", &e.Depth, 0)
		slider("环绕强度", "左右声场的宽度", &e.Width, 0)
		k.row("重置增强", "低音、高音和环绕都回到 0", func() {
			if k.button("enhance-reset", "重置", iconReload, e.Bass == 0 && e.Treble == 0 && e.Depth == 0 && e.Width == 0, false).Clicked() {
				e.Bass, e.Treble, e.Depth, e.Width = 0, 0, 0, 0
				a.shaped()
			}
		})
	})
	k.group("人声与响度", func() {
		slider("人声增强", "居中的人声更靠前、更清楚，伴奏不受影响", &e.Vocal, 0)
		slider("响度均衡", "压低过响的段落、抬起轻柔的段落", &e.Loudness, 0)
	})
	k.group("环境", func() {
		k.row("环境音效", "让声音像在不同的空间里播放", func() {
			labels := make([]string, len(player.Rooms))
			chosen := player.Rooms[0].Label
			for i, r := range player.Rooms {
				labels[i] = r.Label
				if r.ID == e.Room {
					chosen = r.Label
				}
			}
			k.choose("room-select", "环境音效", chosen, labels, func(i int) {
				e.Room = player.Rooms[i].ID
				a.shaped()
			})
		})
	})
}

// eqHeight is how tall the equalizer's sliders run.
const eqHeight = 150

// equalizer is the ten bands, a slider standing up each, over the curve
// they make together.
func (a *app) equalizer(c *ui.Context, ink paneInk) {
	e := &a.settings.Effects
	accent := a.pal.accent
	box := ui.Column(c.Key("eq")).Padding(16, 14, 12, 14).Gap(6)
	box.Children(func() {
		row := ui.Row(c.Key("eq-bands")).AlignItems(ui.Stretch)
		bands := e.Bands
		row.Draw(func(p *ui.Painter, r ui.Rect) {
			// The zero line, and the curve the bands make, filled to it.
			top := r.Y + 20 + knobEQ/2
			span := float32(eqHeight - knobEQ)
			yOf := func(g float64) float32 { return top + span*float32(1-(g+player.EffectRange)/(2*player.EffectRange)) }
			zero := yOf(0)
			p.Fill(ui.Rect{X: r.X + 6, Y: zero - 0.5, W: r.W - 12, H: 1}, ink.text.Alpha(0.1), 0)
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
			p.FillPath(fill, accent.Alpha(0.1))
			p.StrokePath(curve, 2.5, accent)
		})
		row.Children(func() {
			for i := range e.Bands {
				ui.Column(c.Key(i)).Grow(1).Basis(0).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Text(c, gainLabel(e.Bands[i])).FontSize(10.5).FontFeatures("tnum").TextColor(ink.faint).Height(16)
					a.bandSlider(c, i, ink)
					ui.Text(c, freqLabel(player.EffectBands[i])).FontSize(11).FontFeatures("tnum").TextColor(ink.muted)
				})
			}
		})
	})
}

// knobEQ is the size of a band slider's knob.
const knobEQ = 12

// bandSlider is band i, standing up: up lifts it, down cuts it.
func (a *app) bandSlider(c *ui.Context, i int, ink paneInk) {
	e := &a.settings.Effects
	label := freqLabel(player.EffectBands[i]) + "Hz"
	s := ui.SliderBase(c.Key("band"), &e.Bands[i], -player.EffectRange, player.EffectRange).Vertical().
		Size(knobEQ+16, eqHeight).PaddingY(knobEQ / 2).Step(0.5).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
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
		x := r.X + (r.W-2)/2
		p.Fill(ui.Rect{X: x, Y: top, W: 2, H: foot - top}, ink.text.Alpha(0.1), 1)
		y := foot - (foot-top)*g
		mid := (top + foot) / 2
		p.Fill(ui.Rect{X: x, Y: min(y, mid), W: 2, H: max(y, mid) - min(y, mid)}, accent.Alpha(0.6), 1)
		size := float32(knobEQ)
		if hot {
			size += 3
		}
		k := ui.Rect{X: r.X + (r.W-size)/2, Y: y - size/2, W: size, H: size}
		p.Shadow(k, size/2, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.3))
		p.Fill(k, white, size/2)
		p.Stroke(k, accent, size/2, 2)
	})
}

// effectSlider is the slider of an effect, from lo, -1 or 0, to 1, and how
// much beside it: one that goes down and up fills from its middle, and a
// double click puts it back to 0.
func (a *app) effectSlider(c *ui.Context, label string, v *float64, lo float64, ink paneInk) {
	ui.Row(c.Key(label)).Gap(10).AlignItems(ui.Center).Children(func() {
		s := ui.SliderBase(c.Key("slider"), v, lo, 1).Width(180).Height(22).Step(0.01).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
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
		thick := float32(4)
		if s.Hovered() || s.Pressed() {
			thick = 5
		}
		thick = s.AnimateTo("thick", thick, ui.Snappy)
		at, from, accent := float32((*v-lo)/(1-lo)), float32(-lo/(1-lo)), a.pal.accent
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			const knob = 16
			y := r.Y + (r.H-thick)/2
			span := r.W - knob
			x := func(f float32) float32 { return r.X + knob/2 + span*f }
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, ink.press, thick/2)
			if lo < 0 {
				// The middle, where it does nothing.
				p.Fill(ui.Rect{X: x(from) - 0.5, Y: y - 3, W: 1, H: thick + 6}, ink.muted, 0)
			}
			a0, a1 := min(x(from), x(at)), max(x(from), x(at))
			p.Fill(ui.Rect{X: a0, Y: y, W: max(a1-a0, 0.1), H: thick}, accent, thick/2)
			k := ui.Rect{X: x(at) - knob/2, Y: r.Y + (r.H-knob)/2, W: knob, H: knob}
			p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.3))
			p.Fill(k, white, knob/2)
		})
		amount := strconv.Itoa(int(math.Round(*v*100))) + "%"
		if lo < 0 && *v > 0 {
			amount = "+" + amount
		}
		ui.Text(c, amount).FontSize(12).FontFeatures("tnum").TextColor(ink.muted).Width(40).TextAlign(ui.End)
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
