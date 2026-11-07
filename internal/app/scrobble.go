package app

import (
	"time"

	"netease-music-mygo/internal/netease"
)

// scrobble tells NetEase of a song played; tests replace it.
var scrobble = netease.Scrobble

// scrobbleAfter is how long a song is listened to before it counts, as a
// skip a few seconds in should not.
const scrobbleAfter = 30 * time.Second

// listened tells NetEase that the listener played t for played, if they
// asked for their plays to count there: a NetEase song, signed in, heard
// to its end or for half a minute at least.
func (a *app) listened(t netease.Track, played time.Duration, ended bool) {
	if !a.settings.Scrobble || a.profile == nil || t.ID <= 0 || t.ProgramID != 0 {
		return
	}
	if !ended && played < scrobbleAfter {
		return
	}
	seconds := int(played.Seconds())
	if ended && t.Duration > 0 {
		seconds = int(t.Duration / 1000)
	}
	go scrobble(t.ID, 0, seconds, ended)
}
