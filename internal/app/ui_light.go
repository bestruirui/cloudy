package app

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
)

// Now Playing's glow (settings.NpLight), kept as faint as iOS keeps its
// light: what is lit reads by the dark around it, and little more is added.
// The line sung has a soft aura as far as it has been sung, a word held for
// a second or more swells and settles as Apple Music's do, and a bar has a
// haze around what fills it.

// How strongly a line sung and a bar glow, in the alpha of their light.
const (
	lineAura  = 0.62 // a line
	barHaze   = 0.16 // what fills a bar
	barHead   = 0.22 // a bar's head
	wordSwell = 0.45 // a word held, at the height of it
)

// wordLit returns how much of word wd has been sung at pos, from 0 to 1.
func wordLit(wd lyrics.Word, pos time.Duration) float32 {
	return clamp01(float32((pos - wd.Start).Seconds() / max(0.05, (wd.End-wd.Start).Seconds())))
}

// wordHeld returns how much word wd swells at pos, from 0: as AMLL, which
// follows Apple Music's lyrics, has it. Only a word held a second or more
// does, the more the longer it is held, and the last of a line the most: up
// as it begins, and back to nothing by its end, so that a line at rest and
// the many short words of one have no swell at all.
func wordHeld(wd lyrics.Word, last bool, pos time.Duration) float32 {
	held := (wd.End - wd.Start).Seconds()
	if held < 1 {
		return 0
	}
	v := held / 3
	if v > 1 {
		v = math.Sqrt(v)
	} else {
		v = v * v * v
	}
	v *= 0.5
	span := held
	if last {
		v *= 1.5
		span *= 1.2
	}
	t := (pos - wd.Start).Seconds() / span
	if t <= 0 || t >= 1 {
		return 0
	}
	// Quickly up to the middle, and slowly down.
	if t < 0.5 {
		u := 1 - t*2
		return float32(min(v, 0.8) * (1 - u*u))
	}
	u := (t - 0.5) * 2
	return float32(min(v, 0.8) * (1 - u*u*(3-2*u)))
}

// lineGlow is the line sung again in light, blurred around it, as far as it
// has been sung where it has words: those to come are left out of it, for
// nothing to glow ahead of the singer. A word's color keeps a little alpha:
// at none it would fall back to the color of the text around it, in full, as
// a span does.
func (a *app) lineGlow(c *ui.Context, l lyrics.Line, pos time.Duration, st linesStyle) {
	gain := a.glowGain()
	aura := lineAura * gain
	var line ui.Element
	if len(l.Words) > 0 {
		if pos < l.Words[0].Start {
			return
		}
		line = ui.RichText(c.Key("glow")).Children(func() {
			for wi, wd := range l.Words {
				if pos < wd.Start {
					break
				}
				v := aura * wordLit(wd, pos)
				if l.Timed {
					v += wordSwell * gain * wordHeld(wd, wi == len(l.Words)-1, pos)
				}
				ui.Text(c.Key(wi), wd.Text).TextColor(a.glowColor.Alpha(max(min(v, 1), 0.02)))
			}
		})
	} else {
		line = ui.Text(c.Key("glow"), l.Text).TextColor(a.glowColor.Alpha(max(min(aura, 1), 0.02)))
	}
	line.Absolute().Top(0).Left(0).Right(0).FontSize(st.size).FontWeight(st.weight).LineHeight(st.lineHeight).LetterSpacing(-0.4).
		Blur(st.size * 0.22).PassThrough()
}

// drawBarGlow paints the haze of a bar around fill, to a head at cy. A shadow spreads
// what it has over its blur: from something as thin as a bar it needs a
// spread near its blur to show at all, which is what makes these faint
// rather than the alphas alone.
func drawBarGlow(p *ui.Painter, fill ui.Rect, cy float32, color ui.Color, gain float32) {
	if gain <= 0.01 {
		return
	}
	p.Shadow(fill, fill.H/2, 0, 0, 12, 3, color.Alpha(min(barHaze*gain, 1)))
	p.Shadow(ui.Rect{X: fill.X + fill.W, Y: cy}, 0, 0, 0, 12, 5, color.Alpha(min(barHead*gain, 1)))
}

// glowGain is how far the listener has turned the glow up, 1 as it was tuned.
func (a *app) glowGain() float32 { return float32(a.settings.GlowLevel) * 2 }
