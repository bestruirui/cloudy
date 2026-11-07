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

// scrobbleWorks says whether NetEase counts the plays told it. It does not:
// tried on a real account on 2026-11-07, NetEase answers each log of
// /api/feedback/weblog (eapi, weapi or plain) with success and counts none
// of them, in recent plays, the week's ranking or the total. Until a log it
// counts is found, the setting is hidden and nothing is sent.
var scrobbleWorks = false

// listened tells NetEase that the listener played t for played, if they
// asked for their plays to count there: a NetEase song, signed in, heard
// to its end or for half a minute at least.
func (a *app) listened(t netease.Track, played time.Duration, ended bool) {
	if !scrobbleWorks || !a.settings.Scrobble || a.profile == nil || t.ID <= 0 || t.ProgramID != 0 {
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
