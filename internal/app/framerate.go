package app

import "github.com/egoist/mygo/ui"

// The frame rate the listener chooses caps how often the windows draw what
// moves at the display's own rate: transitions, scrolling, the springs of
// opening Now Playing. What moves on its own keeps the rate it was made
// for whatever is chosen, as slow as still looks smooth: the cover turning
// in the bar 15 frames a second, the fog and the smoke 30, the words of a
// line lighting up 25.

// frameRates are the rates the listener chooses among, in the settings:
// 0, each of the display's refreshes, is the default.
var frameRates = []struct {
	fps   int
	label string
}{
	{0, "跟随显示器"},
	{30, "30 帧（省电）"},
	{60, "60 帧"},
	{120, "120 帧"},
	{144, "144 帧"},
}

// setFrameRate takes the listener's choice, 0 for the display's rate. A
// rate above any display's, as the setting was once kept for it, is the
// display's.
func setFrameRate(fps int) {
	if fps < 0 || fps > 500 {
		fps = 0
	}
	ui.SetMaxFrameRate(float32(fps))
}

// frameRateLabel names the rate fps.
func frameRateLabel(fps int) string {
	for _, r := range frameRates {
		if r.fps == fps {
			return r.label
		}
	}
	return frameRates[0].label
}
