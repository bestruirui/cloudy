package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// recentState is the songs played in Cloudy on this computer, as the
// 最近播放 page lists them: kept in history.json, as history.go does.
type recentState struct {
	played []playedSong
	loaded bool // whether history.json was read
	// sure is the 清空 button asked again, its next click clearing.
	sure bool
}

// recentPage lists the songs played on this computer, the one playing and
// then the latest first. It needs no account: the songs are this
// computer's own.
func (a *app) recentPage(c *ui.Context) {
	r := &a.recent
	tracks := a.recentSongs()
	header := func(c *ui.Context) {
		p := a.pal
		sub := "在这台电脑上用 Cloudy 听过的歌曲"
		if len(tracks) > 0 {
			sub = fmt.Sprintf("在这台电脑上用 Cloudy 听过的 %d 首", len(tracks))
		}
		a.heading(c, "最近播放", sub)
		if len(tracks) > 0 {
			ui.Row(c).Gap(10).Margin(0, 0, 18, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
					a.playList(tracks, false)
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					a.playList(tracks, true)
				}
				if len(r.played) == 0 {
					return
				}
				label, color := "清空记录", p.text
				if r.sure {
					label, color = "再点一次清空", p.accent
				}
				clear := pillButton(c.Key("clear"), iconTrash, label, p.hover.Over(p.bg), color)
				if clear.Clicked() {
					if r.sure {
						a.clearHistory()
					}
					r.sure = !r.sure
				} else if r.sure && !clear.Hovered() {
					// Moved away from, it asks again from the start.
					r.sure = false
				}
			})
		}
	}
	if len(tracks) == 0 {
		a.centered(c, "还没有播放记录：在 Cloudy 里听过的歌会出现在这里", nil)
		return
	}
	a.toolRows(c, tracks, true, header)
}
