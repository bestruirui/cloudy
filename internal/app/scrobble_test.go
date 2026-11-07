package app

import (
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

// Plays count on NetEase only when the listener turned it on, signed in,
// for NetEase songs heard to their end or for half a minute: a skip, an
// episode or a file of their own does not.
func TestScrobble(t *testing.T) {
	old := scrobble
	defer func() { scrobble = old }()
	defer func(works bool) { scrobbleWorks = works }(scrobbleWorks)
	got := make(chan [2]int64, 8)
	scrobble = func(id, source int64, seconds int, ended bool) error {
		got <- [2]int64{id, int64(seconds)}
		return nil
	}
	a := newTestApp()
	a.settings = defaultSettings()
	song := netease.Track{ID: 7, Duration: 200000}
	a.profile = &netease.Profile{UserID: 1}
	a.listened(song, time.Minute, false)
	select {
	case <-got:
		t.Fatal("told NetEase with the setting off")
	case <-time.After(50 * time.Millisecond):
	}
	a.settings.Scrobble = true
	// Hidden, as NetEase counts none of them: not told even when turned on.
	a.listened(song, time.Minute, true)
	select {
	case <-got:
		t.Fatal("told NetEase while the setting is hidden")
	case <-time.After(50 * time.Millisecond):
	}
	scrobbleWorks = true
	for _, c := range []struct {
		tr     netease.Track
		played time.Duration
		ended  bool
	}{
		{song, 10 * time.Second, false},
		{netease.Track{ID: 8, ProgramID: 3}, time.Minute, true},
		{netease.Track{ID: -9, LocalPath: "a.mp3"}, time.Minute, true},
	} {
		a.listened(c.tr, c.played, c.ended)
	}
	a.listened(song, 40*time.Second, false)
	a.listened(song, 150*time.Second, true)
	// Each goes on its own, in either order.
	want := map[[2]int64]bool{{7, 40}: true, {7, 200}: true}
	for range len(want) {
		select {
		case g := <-got:
			if !want[g] {
				t.Errorf("told %v", g)
			}
			delete(want, g)
		case <-time.After(time.Second):
			t.Fatalf("not told %v", want)
		}
	}
	select {
	case g := <-got:
		t.Errorf("also told %v", g)
	case <-time.After(50 * time.Millisecond):
	}
}
