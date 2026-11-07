package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// recentState is the songs the listener played lately, on any of their
// devices, as the 最近播放 page lists them.
type recentState struct {
	tracks  []netease.Track
	loaded  time.Time
	loading bool
	err     string
}

// recentLimit is how many of the songs played lately the page lists:
// NetEase keeps a few hundred.
const recentLimit = 300

// loadRecent loads the songs played lately, again once they are a minute
// old, as the page opens: they change as the listener plays.
func (a *app) loadRecent(force bool) {
	r := &a.recent
	if a.profile == nil || r.loading || !force && time.Since(r.loaded) < time.Minute {
		return
	}
	r.loading, r.err = true, ""
	gen := a.accountGen
	go func() {
		plays, err := recentPlays(recentLimit)
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			r.loading = false
			if err != nil {
				r.err = "最近播放加载失败"
				return
			}
			tracks := make([]netease.Track, 0, len(plays))
			seen := map[int64]bool{}
			for _, p := range plays {
				if !seen[p.Track.ID] {
					seen[p.Track.ID] = true
					tracks = append(tracks, p.Track)
				}
			}
			r.tracks, r.loaded = tracks, time.Now()
		})
	}()
}

// recentPage lists the songs played lately, the latest first.
func (a *app) recentPage(c *ui.Context) {
	r := &a.recent
	if a.profile == nil {
		a.centered(c, "登录后可以看到在各个设备上最近播放的歌曲", nil)
		return
	}
	header := func(c *ui.Context) {
		p := a.pal
		sub := "在手机、电脑和网页上听过的歌曲"
		if len(r.tracks) > 0 {
			sub = fmt.Sprintf("最近听过的 %d 首，包括在其他设备上播放的", len(r.tracks))
		}
		a.heading(c, "最近播放", sub)
		if len(r.tracks) > 0 {
			ui.Row(c).Gap(10).Margin(0, 0, 18, 0).Children(func() {
				if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
					a.playList(r.tracks, false)
				}
				if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
					a.playList(r.tracks, true)
				}
				if pillButton(c.Key("refresh"), iconReload, "刷新", p.hover.Over(p.bg), p.text).Clicked() {
					a.loadRecent(true)
				}
			})
		}
	}
	switch {
	case len(r.tracks) == 0 && r.err != "":
		a.centered(c, r.err, func() { a.loadRecent(true) })
	case len(r.tracks) == 0 && (r.loading || r.loaded.IsZero()):
		a.trackSkeleton(c, a.pageShimmer(c), true, header)
	case len(r.tracks) == 0:
		a.centered(c, "还没有播放记录", nil)
	default:
		a.toolRows(c, r.tracks, true, header)
	}
}
