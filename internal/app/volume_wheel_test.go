package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// One wheel notch over the volume changes it by one percent, not a quarter.
func TestVolumeWheelStepsOnePercent(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.volume = 0.5
	if !a.scrollVolume(ui.InputEvent{Kind: ui.InputScroll, DY: 100}) {
		t.Fatal("the wheel was not taken")
	}
	if math.Abs(a.volume-0.49) > 1e-9 {
		t.Fatalf("volume %v, want 0.49", a.volume)
	}
	a.scrollVolume(ui.InputEvent{Kind: ui.InputScroll, DY: -100})
	if math.Abs(a.volume-0.50) > 1e-9 {
		t.Fatalf("volume %v, want 0.50", a.volume)
	}
}
