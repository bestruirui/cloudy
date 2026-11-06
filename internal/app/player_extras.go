package app

import (
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// toggleLyricTrans shows or hides the translations of lyrics, and says
// which, as the player bar's button does.
func (a *app) toggleLyricTrans() {
	a.settings.LyricTrans = !a.settings.LyricTrans
	a.changed()
	if a.settings.LyricTrans {
		a.toast = "已显示歌词翻译"
	} else {
		a.toast = "已隐藏歌词翻译"
	}
	a.redraw()
}

// addToPlaylistMenu opens, above the player bar's button b, the playlists
// the song tr can be added to; signed out, it asks the listener to sign in.
func (a *app) addToPlaylistMenu(b ui.Element, tr *netease.Track) {
	if a.profile == nil {
		a.toast = "登录后才能添加到歌单"
		a.openLogin()
		a.redraw()
		return
	}
	song := *tr
	acts := a.playlistActions(&song, 0)
	if len(acts) == 0 || len(acts[0].sub) == 0 {
		return
	}
	// The submenu's groups, as a menu of their own: new, then the lists.
	a.menuBelow(b, false, acts[0].sub, a.songHead(&song))
}
