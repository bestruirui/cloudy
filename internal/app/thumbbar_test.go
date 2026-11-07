package app

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"

	"netease-music-mygo/internal/netease"
)

// Each glyph is a PNG of its own, ink on nothing, of the size Windows
// scales from.
func TestThumbGlyphs(t *testing.T) {
	ink := color.NRGBA{255, 255, 255, 255}
	seen := map[string]glyph{}
	for g := glyphPrev; g <= glyphHeartFull; g++ {
		data := thumbGlyph(g, ink)
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("glyph %d: %v", g, err)
		}
		if b := img.Bounds(); b.Dx() != thumbSize || b.Dy() != thumbSize {
			t.Errorf("glyph %d is %v", g, b)
		}
		if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
			t.Errorf("glyph %d: corner not clear", g)
		}
		if _, _, _, a := img.At(thumbSize/2, thumbSize/2).RGBA(); g != glyphPause && g != glyphHeart && a == 0 {
			t.Errorf("glyph %d: middle not drawn", g)
		}
		if other, ok := seen[string(data)]; ok {
			t.Errorf("glyphs %d and %d are the same", other, g)
		}
		seen[string(data)] = g
	}
}

// Before a song is chosen there are no buttons to show, and no song to
// read them from.
func TestThumbsWithoutSong(t *testing.T) {
	a := newTestApp()
	if a.hasSong() {
		t.Fatal("a song before any was cued")
	}
	a.pl.Cue([]netease.Track{{ID: 7, Name: "七"}})
	if !a.hasSong() {
		t.Fatal("no song once cued")
	}
}

// The heart not liked is an outline, clear inside; the liked one full.
func TestThumbHearts(t *testing.T) {
	ink := color.NRGBA{255, 255, 255, 255}
	at := func(g glyph) uint32 {
		img, _ := png.Decode(bytes.NewReader(thumbGlyph(g, ink)))
		_, _, _, a := img.At(thumbSize/2, thumbSize/2).RGBA()
		return a
	}
	if a := at(glyphHeart); a != 0 {
		t.Errorf("outline heart drawn inside: alpha %d", a)
	}
	if a := at(glyphHeartFull); a == 0 {
		t.Error("full heart clear inside")
	}
}

// The window is titled with the song and its artists, as NetEase's is.
func TestSongTitle(t *testing.T) {
	if got := songTitle("痴人说梦", "HOYO-MiX"); got != "痴人说梦 - HOYO-MiX" {
		t.Errorf("title %q", got)
	}
	if got := songTitle("一", ""); got != "一" {
		t.Errorf("title without artists %q", got)
	}
}
