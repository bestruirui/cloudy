package app

import (
	"runtime"
	"testing"
)

// A font id kept in settings but no longer on offer reads as the default,
// rather than as a family no one has.
func TestFontFamilyOfUnknownFont(t *testing.T) {
	a := newTestApp()
	a.settings.Font = "gone"
	if got := a.fontFamily(); got != defaultFontFamily() {
		t.Errorf("family = %q, want the default %q", got, defaultFontFamily())
	}
	a.settings.Font = "simsun"
	if got := a.fontFamily(); got != "SimSun" {
		t.Errorf("family = %q, want SimSun", got)
	}
}

// Segoe UI drew Windows's Latin text and left Han characters to fallbacks,
// so the default has to name a Chinese family there.
func TestDefaultFontNamesChineseFamilyOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows lacks a Chinese default")
	}
	if defaultFontFamily() == "" {
		t.Error("the default font leaves Han characters to the system's fallback")
	}
}

// A font beyond those on offer is kept by its family or file, and the
// default draws what it lacks.
func TestCustomFonts(t *testing.T) {
	f := fontOf("sys:Segoe Print")
	if f.label != "Segoe Print" || f.family != "Segoe Print" || !f.custom() {
		t.Fatalf("system font: %+v", f)
	}
	f = fontOf("file:MyFont.ttf")
	if f.label != "MyFont" || f.imported != "MyFont.ttf" || f.family != importedFamily("MyFont.ttf") {
		t.Fatalf("imported font: %+v", f)
	}
	if fontOf("file:../evil.ttf").id != "" || fontOf("sys:").id != "" {
		t.Fatal("a font id out of bounds was taken")
	}
	if fontOf("wenkai").custom() {
		t.Fatal("a font on offer counts as custom")
	}
	a := newTestApp()
	a.settings.Font = "sys:Segoe Print"
	got := a.fontFamily()
	if def := defaultFontFamily(); def != "" && got != "Segoe Print, "+def {
		t.Fatalf("family %q has no fallback", got)
	}
}
