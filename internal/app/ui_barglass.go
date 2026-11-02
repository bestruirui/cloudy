package app

import (
	"sync"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// barGlass is the glass of the player bar: clearer and bending what shows
// through more than the glass plugin's, whose refraction is fixed. It follows
// the plugin's optics (its lens, normal and tone, see glass.hlsl), with these
// to tell it from them:
//
//   - it hardly blurs, so that what is under the bar can be read through its
//     flat middle;
//   - its curve reaches further in, on a rounder profile, and bends further,
//     so that the edges of the capsule visibly swim;
//   - red and blue bend a little apart, as through a thick lens;
//   - in place of a tint over everything, the lightness of what shows through
//     is squeezed: dark stays as it is, and only what is bright, which would
//     drown the bar's own words, comes down.
//
// clear and bend, from 0 to 1, are the listener's: clearer, it blurs less and
// lets more of what is bright through; bent more, the edge bends further, and
// its curve reaches further in. At 0.5 they are the glass as it was tuned.
type barGlass struct {
	dark        bool
	clear, bend float32
}

// How the glass of the bar looks, whatever the listener chose.
const (
	barGlassSpread  = 0.05 // how far apart red and blue bend, of the bend
	barGlassRim     = 0.5  // how lit the rim is
	barGlassRimWide = 1.1  // how wide it is, in DIPs
)

const barGlassSource = `package main

var Bend float
var Bezel float
var Spread float
var Rim float
var RimWide float
var Tone vec4
var Ink vec4

// edge returns how far pos is inside the capsule, and in xy, the way out of
// it, smoothed around its ends.
func edge(pos, half vec2) vec3 {
	q := pos - half
	r := half.y
	a := abs(q) - vec2(half.x-r, 0)
	m := max(a, vec2(0))
	inside := r - length(m)
	n := vec2(0)
	if m.x > 0.0 || m.y > 0.0 {
		n = sign(q) * m / length(m)
	}
	return vec3(n, max(inside, 0))
}

// lens returns how far the surface bends what shows through at x, 0 at the
// edge of the curve to 1 where it starts, the glass plugin's Snell's law over
// a surface that is a circle's rather than Apple's squircle: rounder, it
// keeps bending further in.
func lens(x float) float {
	u := clamp(1.0-x, 0.0, 1.0)
	v := sqrt(max(1.0-u*u, 0.0001))
	n := sqrt(u*u + v*v)
	s := u / n
	c := v / n
	st := s / 1.5
	ct := sqrt(1.0 - st*st)
	return (s*ct - c*st) / (c*ct + s*st) / 1.118034
}

func tone(c vec3) vec3 {
	l := dot(c, vec3(0.2126, 0.7152, 0.0722))
	t := Tone.x + (Tone.y-Tone.x)*(1.0-pow(max(1.0-l, 0.0), Tone.z))
	k := (Tone.y - Tone.x) * Tone.w
	return clamp(vec3(t)+(c-vec3(l))*k, vec3(0), vec3(1))
}

func Fragment(dst vec4, pos vec2) vec4 {
	half := rectSize() / 2.0
	e := edge(pos, half)
	bezel := half.y * Bezel
	n := e.xy
	t := e.z
	bend := vec2(0)
	if t < bezel {
		bend = n * Bend * lens(t/bezel)
	}
	r := backdropSample(pos - bend*(1.0+Spread)).r
	g := backdropSample(pos - bend).g
	b := backdropSample(pos - bend*(1.0-Spread)).b
	c := tone(vec3(r, g, b))
	// Under what plays and the buttons at the end, what shows through keeps
	// to itself: words over words can't be read, however clear the glass.
	width := rectSize().x
	side := max(width*0.5-220.0, 130.0)
	under := max(1.0-smoothstep(side*0.72, side*1.02, pos.x), 0.7*smoothstep(width-side*0.6, width-side*0.35, pos.x))
	c = mix(c, Ink.rgb, Ink.a*under)
	// The rim, a hairline lit where it faces up or down, and shaded where
	// it faces the sides.
	if Rim > 0.0 && t < 3.0*RimWide {
		x := t / RimWide
		c += (vec3(abs(n.y)) - c) * Rim * exp(-x*x)
	}
	return vec4(c, 1)
}
`

var barGlassShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(barGlassSource) })

// PaintMaterial paints the glass over box, a capsule by radii. On the CPU the
// shader, which runs interpreted, would take a good part of a second a frame:
// there it is the plugin's glass, which is drawn natively.
func (g barGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if !p.Accelerated() {
		tint := ui.RGBA(255, 255, 255, 0.24)
		if g.dark {
			tint = ui.RGBA(0, 0, 0, 0.32)
		}
		glass.Paint(p, box, radii[0], glass.Glass{Tint: tint})
		return
	}
	// The lightness of black and white through it, how it bends between
	// them, and how much of its colors it keeps.
	tone := [4]float32{0.62 - 0.4*g.clear, 1, 1.2, 1.35}
	if g.dark {
		tone = [4]float32{0.03, 0.32 + 0.48*g.clear, 1.5, 2.4}
	}
	blur := 2.8 * (1 - g.clear) * (1 - g.clear)
	ink := [4]float32{0.97, 0.97, 0.98, 0.85}
	if g.dark {
		ink = [4]float32{0.05, 0.05, 0.06, 0.85}
	}
	barGlassShader().Draw(p, box, shader.Options{Radius: radii[0], BackdropBlur: blur, Uniforms: map[string]any{
		"Bend": 144 * g.bend, "Bezel": 0.33 + 0.66*g.bend, "Spread": barGlassSpread,
		"Rim": barGlassRim, "RimWide": barGlassRimWide, "Tone": tone[:], "Ink": ink[:],
	}})
}
