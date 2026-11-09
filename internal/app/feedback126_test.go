package app

import (
	"testing"
	"time"
)

// Now Playing has a button for full screen, beside the window's buttons.
func TestNowPlayingHasFullScreenButton(t *testing.T) {
	_, tt, _ := coverApp(t)
	if _, ok := tt.Find("全屏 (F11)"); !ok {
		t.Fatal("no full screen button in Now Playing")
	}
}

// The first time Now Playing opens in a run, a bubble tells of F11, and
// goes after a while, not to come back as Now Playing opens again.
func TestFullScreenHintOnce(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.fullHintAt = time.Time{} // a run that has not shown it yet
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	if !tt.HasText("按 F11 全屏，Esc 退出") {
		t.Fatal("no F11 hint as Now Playing first opened")
	}
	tt.Advance(fullHintFor + time.Second)
	tt.Frame()
	if tt.HasText("按 F11 全屏，Esc 退出") {
		t.Fatal("the F11 hint stayed")
	}
	a.nowPlaying = false
	tt.Advance(time.Second)
	a.nowPlaying = true
	tt.Advance(time.Second)
	if tt.HasText("按 F11 全屏，Esc 退出") {
		t.Fatal("the F11 hint came back in the same run")
	}
}

// Following the system, the accent is the desktop's, and the app's red
// where the desktop has none; a color chosen is that color.
func TestChosenAccent(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	if got := a.chosenAccent(); got != "" {
		t.Fatalf("default accent %q, want the app's red", got)
	}
	a.settings.Accent = "#3D7BD9"
	if got := a.chosenAccent(); got != "#3D7BD9" {
		t.Fatalf("chosen accent %q", got)
	}
	a.settings.Accent = accentSystem
	if got := a.chosenAccent(); got != "" {
		t.Fatalf("system accent with none known: %q", got)
	}
	a.sysAccent = "#0078D4"
	if got := a.chosenAccent(); got != "#0078D4" {
		t.Fatalf("system accent %q", got)
	}
}

// Closing Now Playing leaves full screen.
func TestClosingNowPlayingLeavesFullScreen(t *testing.T) {
	a, tt, _ := coverApp(t)
	a.npFull = true // as setNpFull would, with a window
	a.nowPlaying = false
	tt.Frame()
	if a.npFull {
		t.Fatal("still full screen with Now Playing closed")
	}
}
