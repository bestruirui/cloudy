package app

import "time"

// What moves on its own, as the cover turning in the bar, the fog behind
// Now Playing or the words of a line lighting up as they are sung, is drawn
// at the frame rate the listener chose, rather than at each of the
// display's refreshes, which on a 144 or 240 Hz display would keep the GPU
// busy for motion the eye does not see. What the listener moves, scrolls
// and the app's transitions go at the display's own rate whatever it is.

// frameRateDisplay is the setting for each of the display's refreshes.
const frameRateDisplay = 1000

// frameRates are the rates the listener chooses among, in the settings.
var frameRates = []struct {
	fps   int
	label string
}{
	{30, "30 帧（省电）"},
	{60, "60 帧"},
	{120, "120 帧"},
	{144, "144 帧"},
	{frameRateDisplay, "跟随显示器"},
}

// motionFPS is the frame rate of what moves on its own, as set: the
// window paints it again no faster than the display refreshes.
var motionFPS float32 = 60

// setFrameRate takes the listener's choice, 0 for the default.
func setFrameRate(fps int) {
	if fps <= 0 {
		fps = 60
	}
	motionFPS = float32(fps)
}

// motionFrame is how long a frame of what moves on its own lasts.
func motionFrame() time.Duration {
	return time.Duration(float32(time.Second) / motionFPS)
}

// frameRateLabel names the rate fps.
func frameRateLabel(fps int) string {
	if fps <= 0 {
		fps = 60
	}
	for _, r := range frameRates {
		if r.fps == fps {
			return r.label
		}
	}
	return frameRates[1].label
}
