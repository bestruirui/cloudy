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
