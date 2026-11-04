package app

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Radios play on without end: the personal FM, in one of its modes, as
// "fm:<mode>", and the heart mode of the liked songs, as "heart".

const heartRadio = "heart"

// Tests replace the personal feed without signing in to a real account.
var personalFM = netease.PersonalFM

var fmModes = []struct{ mode, label string }{
	{netease.FMDefault, "推荐"},
	{netease.FMFamiliar, "熟悉"},
	{netease.FMExplore, "探索"},
}

func fmRadio(mode string) string { return "fm:" + mode }

// radioLabel names a radio as the listener knows it.
func radioLabel(name string) string {
	if name == heartRadio {
		return "心动模式"
	}
	if isRoam(name) {
		return "风格漫游 · " + strings.ReplaceAll(strings.TrimPrefix(name, stylePrefix), "|", " · ")
	}
	for _, m := range fmModes {
		if name == fmRadio(m.mode) {
			return "私人 FM · " + m.label
		}
	}
	return "私人 FM"
}

func playable(tracks []netease.Track) []netease.Track {
	var out []netease.Track
	for _, t := range tracks {
		if t.Playable() {
			out = append(out, t)
		}
	}
	return out
}

// likedPlaylist returns the ID of the listener's liked songs, or 0.
func (a *app) likedPlaylist() int64 {
	for _, pl := range a.mine {
		if pl.SpecialType == 5 {
			return pl.ID
		}
	}
	return 0
}

// radioFeed returns what gives the radio name more tracks: it loads them
// on a goroutine, and gives them to got on the main thread, none if it
// cannot.
func (a *app) radioFeed(name string) func(got func([]netease.Track)) {
	return func(got func([]netease.Track)) {
		var fetch func() ([]netease.Track, error)
		switch {
		case strings.HasPrefix(name, "fm:"):
			mode := strings.TrimPrefix(name, "fm:")
			fetch = func() ([]netease.Track, error) { return personalFM(mode) }
		case isRoam(name):
			label := strings.TrimPrefix(name, stylePrefix)
			if a.roam == nil {
				a.roam = &roamer{}
			}
			roam := a.roam
			fetch = func() ([]netease.Track, error) { return roam.next(label) }
		case name == heartRadio:
			// More like the last song queued, from the song it began with.
			q, pid := a.pl.Queue(), a.likedPlaylist()
			if len(q) == 0 || pid == 0 {
				got(nil)
				return
			}
			start, last := q[0].ID, q[len(q)-1].ID
			fetch = func() ([]netease.Track, error) { return netease.HeartMode(last, start, pid) }
		default:
			got(nil)
			return
		}
		go func() {
			// The FM gives a song or three at a time, some of them VIP
			// songs the listener may not play: ask again for those.
			var tracks []netease.Track
			for tries := 0; tries < 3 && len(tracks) == 0; tries++ {
				more, err := fetch()
				if err != nil {
					break
				}
				tracks = playable(more)
			}
			a.post(func() { got(tracks) })
		}()
	}
}

// playFM plays the personal FM in the mode chosen, or goes on playing it.
func (a *app) playFM() {
	name := fmRadio(fmModes[a.fmMode].mode)
	if a.pl.Radio() == name {
		if !a.pl.Playing() {
			a.pl.TogglePlay()
		}
		return
	}
	a.startRadio(name, "私人 FM 暂时无法播放，请稍后再试")
}

// startRadio plays the radio name, or says it can't, as failed.
func (a *app) startRadio(name, failed string) {
	req := a.requestRadio(name)
	feed := a.radioFeed(name)
	feed(func(tracks []netease.Track) {
		if !a.acceptRadio(req) {
			return
		}
		if len(tracks) == 0 {
			a.toast = failed
			return
		}
		a.pl.PlayRadio(name, tracks, feed)
	})
}

// A request belongs to one login and one playback selection. Even two
// requests for the same station must not accept one another's results.
type radioRequest struct{ gen, playback, account int }

func (a *app) requestRadio(name string) radioRequest {
	a.radioGen++
	a.radioStarting = name
	return radioRequest{a.radioGen, a.pl.RadioGeneration(), a.accountGen}
}

func (a *app) cancelRadioStart() {
	a.radioGen++
	a.radioStarting = ""
}

func (a *app) acceptRadio(req radioRequest) bool {
	if req.gen != a.radioGen {
		return false
	}
	a.radioStarting = ""
	return req.playback == a.pl.RadioGeneration() && req.account == a.accountGen
}

// chooseFMMode leaves style roaming, including a pending first response.
func (a *app) chooseFMMode(mode int) {
	a.fmMode = mode
	if a.radioStarting == fmRadio(fmModes[mode].mode) {
		return
	}
	on := a.pl.Radio()
	pending := a.radioStarting != ""
	a.cancelRadioStart()
	if strings.HasPrefix(on, "fm:") || isRoam(on) || pending {
		a.playFM()
	}
}

// exitRadio stops the radio playing, but not the song it plays.
func (a *app) exitRadio() {
	a.cancelRadioStart()
	name := a.pl.Radio()
	if name == "" {
		return
	}
	a.pl.EndRadio()
	label := radioLabel(name)
	switch {
	case isRoam(name):
		label = "风格漫游"
	case name != heartRadio:
		label = "私人 FM"
	}
	a.toast = "已退出" + label
}

// trashFM skips the song playing on the FM, which plays it no more.
func (a *app) trashFM() {
	tr := a.pl.Current()
	if tr == nil {
		return
	}
	id, roaming := tr.ID, isRoam(a.pl.Radio())
	a.pl.Next()
	if !roaming {
		go netease.FMTrash(id)
	}
}

// startHeartMode plays songs like one the listener likes: the one playing,
// if they like it, or else one of their liked songs at random.
func (a *app) startHeartMode() {
	if a.profile == nil {
		a.showMain()
		a.openLogin()
		return
	}
	pid := a.likedPlaylist()
	if pid == 0 {
		a.toast = "喜欢一些歌曲后，即可开启心动模式"
		return
	}
	if a.pl.Radio() == heartRadio {
		a.nowPlaying = true
		return
	}
	var seed *netease.Track
	fromPlaying := false
	if cur := a.pl.Current(); cur != nil && a.liked[cur.ID] {
		s := *cur
		seed, fromPlaying = &s, true
	}
	liked := a.playlists[pid]
	req := a.requestRadio(heartRadio)
	go func() {
		if seed == nil {
			if liked == nil {
				liked, _ = netease.PlaylistDetail(pid)
			}
			if liked != nil {
				if ok := playable(liked.Tracks); len(ok) > 0 {
					seed = &ok[rand.IntN(len(ok))]
				}
			}
		}
		var more []netease.Track
		var err error
		if seed != nil {
			more, err = netease.HeartMode(seed.ID, seed.ID, pid)
		}
		a.post(func() {
			if !a.acceptRadio(req) {
				return
			}
			if seed == nil || err != nil {
				a.toast = "心动模式暂时无法开启，请稍后再试"
				return
			}
			more = playable(more)
			if fromPlaying && a.pl.Current() != nil && a.pl.Current().ID == seed.ID {
				a.pl.RadioFrom(heartRadio, more, a.radioFeed(heartRadio))
				return
			}
			a.pl.PlayRadio(heartRadio, append([]netease.Track{*seed}, more...), a.radioFeed(heartRadio))
		})
	}()
}

// fmPage is the personal FM: what it plays, large, over its cover's
// colors, with the FM's own controls, and heart mode below.
func (a *app) fmPage(c *ui.Context) {
	p := a.pal
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("fm")).TrackScroll(&a.pageScroll).Grow(1).Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).Children(func() {
		a.startMark(c)
		defer a.endMark(c)
		// Keep a pending choice visible while the previous station plays on.
		chosen := a.radioStarting
		if chosen == "" {
			chosen = a.pl.Radio()
		}
		for i, m := range fmModes {
			if chosen == fmRadio(m.mode) {
				a.fmMode = i
			}
		}
		ui.Row(c).AlignItems(ui.End).Children(func() {
			ui.Box(c).Grow(1).Children(func() { a.heading(c, "私人 FM", "无尽推荐") })
			mode := a.fmMode
			ui.Box(c).Margin(0, 0, 22, 0).Children(func() {
				labels := make([]string, len(fmModes))
				for i, m := range fmModes {
					labels[i] = m.label
				}
				active := !isRoam(chosen)
				a.segmentedControl(c.Key("fm-mode"), &mode, labels, active, a.chooseFMMode)
			})
		})
		a.fmCard(c)
		a.section(c, "心动模式")
		a.heartCard(c, p)
		a.roamSection(c)
	})
}

// fmCard shows the song the FM plays, or how to start it.
func (a *app) fmCard(c *ui.Context) {
	pl := a.pl
	onFM := strings.HasPrefix(pl.Radio(), "fm:") || isRoam(pl.Radio())
	var tr *netease.Track
	if onFM {
		tr = pl.Current()
	}
	var art *artwork
	if tr != nil {
		art = a.art.get(netease.Sized(tr.Album.PicURL, 600), true)
	}
	card := ui.Box(c.Key("fm-card")).Height(320).Radius(26).Background(ui.Hex("#232328")).TextColor(white).
		Shadow(0, 18, 44, -16, a.pal.cardShade)
	in := artIn(card, backdrop(art))
	card.Children(func() {
		if in < 1 {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(26).
				LinearGradient(ui.LinearGradient{Angle: 135, Oklab: true, From: ui.Oklch(0.42, 0.14, 20), To: ui.Oklch(0.3, 0.1, 300)})
		}
		artImage(c, backdrop(art), 26, in)
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(26).
			LinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
				{Color: ui.RGBA(0, 0, 0, 0.35), At: 0},
				{Color: ui.RGBA(0, 0, 0, 0.1), At: 1},
			}})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(36, 40).Gap(40).AlignItems(ui.Center).Children(func() {
			a.fmCovers(c, tr)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				label := "私人 FM · " + fmModes[a.fmMode].label
				if onFM {
					label = radioLabel(pl.Radio())
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconRadio).Size(14, 14).TextColor(white.Alpha(0.75))
					ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(white.Alpha(0.75))
				})
				if tr == nil {
					ui.Text(c, "听见更多好音乐").FontSize(30).FontWeight(800).LetterSpacing(-0.6)
					ui.Text(c, "根据你的口味，一首接一首，不停推荐").FontSize(13.5).TextColor(white.Alpha(0.75))
					ui.Row(c).Margin(18, 0, 0, 0).Children(func() {
						label := "开始收听"
						if a.radioStarting != "" && strings.HasPrefix(a.radioStarting, "fm:") {
							label = "正在调频…"
						}
						if pillButton(c.Key("start"), iconPlay, label, white, ui.Hex("#1d1d1f")).Clicked() {
							a.playFM()
						}
					})
					return
				}
				ui.Text(c, tr.Name).FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(2).LineHeight(1.2)
				switch {
				case pl.Loading():
					ui.Text(c, "正在缓冲…").FontSize(14).TextColor(white.Alpha(0.7)).SingleLine()
				case pl.Err() != "":
					ui.Text(c, pl.Err()).FontSize(14).TextColor(white).SingleLine()
				default:
					a.artistLinks(c, tr, white.Alpha(0.7), white, true).FontSize(14).SingleLine()
				}
				ui.Row(c).Gap(14).AlignItems(ui.Center).Margin(20, 0, 0, -8).Children(func() {
					if a.profile != nil {
						if iconButton(c.Key("trash"), iconTrash, 20, white.Alpha(0.8), "不喜欢").Clicked() {
							a.trashFM()
						}
					}
					a.likeButton(c.Key("like"), tr, 20, white.Alpha(0.8))
					play := ui.ButtonBase(c.Key("play")).Size(56, 56).Center().Radius(999).Background(white).TextColor(ui.Hex("#1d1d1f")).Label("播放/暂停")
					pose := ui.Pose{}
					if play.Pressed() {
						pose.Scale = 0.92
					} else if play.Hovered() {
						pose.Scale = 1.05
					}
					play.AnimatePose(pose, ui.Snappy)
					play.Children(func() { playPause(c, play, pl.Playing(), 22) })
					if play.Clicked() {
						pl.TogglePlay()
					}
					if iconButton(c.Key("next"), iconNext, 20, white.Alpha(0.8), "下一首").Clicked() {
						pl.Next()
					}
				})
			})
		})
	})
}

// fmCoverKey keeps each queued cover's identity as it moves through the stack.
type fmCoverKey struct {
	id    int64
	index int
}

type fmCoverItem struct {
	key   fmCoverKey
	track netease.Track
	until time.Time
}

// Keep the departing cover above its successor until it has faded away.
type fmCoverStack struct {
	current fmCoverItem
	leaving []fmCoverItem
}

const fmCoverExit = 340 * time.Millisecond

var fmCoverMove = ui.Animation{Spring: ui.SpringOptions{Duration: 600 * time.Millisecond}}
var fmCoverLeave = ui.Animation{Duration: fmCoverExit, Ease: ui.CubicBezier(0.3, 0, 0.7, 1)}
var fmCoverEnter = ui.Animation{Duration: 420 * time.Millisecond, Delay: 80 * time.Millisecond, Ease: ui.CubicBezier(0.22, 1, 0.36, 1)}

// fmCovers lets the next queued cover come forward as the current one leaves.
func (a *app) fmCovers(c *ui.Context, tr *netease.Track) {
	const size = 232
	ui.Box(c).Size(size+36, size).Shrink(0).Children(func() {
		if tr == nil {
			a.fmCoversState = fmCoverStack{}
			ui.Box(c.Key("empty")).Absolute().Top(0).Left(0).Size(size, size).Radius(14).
				Background(white.Alpha(0.12)).Center().Children(func() {
				ui.Icon(c, iconRadio).Size(64, 64).TextColor(white.Alpha(0.85))
			})
			return
		}
		queue := a.pl.Queue()
		order, at := a.pl.PlayOrder()
		if at < 0 || at >= len(order) {
			return
		}
		current, next := order[at], a.pl.NextIndex()
		key := fmCoverKey{tr.ID, current}
		state, now := &a.fmCoversState, c.Now()
		if state.current.key != key {
			if state.current.track.ID != 0 {
				old := state.current
				old.until = now.Add(fmCoverExit)
				state.leaving = append(state.leaving, old)
			}
			state.current = fmCoverItem{key: key, track: *tr}
		}
		// A quick reversal can put a departing cover back into the queue.
		kept := state.leaving[:0]
		for _, item := range state.leaving {
			if !now.Before(item.until) || item.key == key || next >= 0 && item.key == (fmCoverKey{queue[next].ID, next}) {
				continue
			}
			kept = append(kept, item)
			c.After(item.until.Sub(now))
		}
		clear(state.leaving[len(kept):])
		state.leaving = kept
		draw := func(item fmCoverItem, pose ui.Pose, animation ui.Animation, label string) {
			// Fixed geometry keeps the cover on the compositor throughout the move.
			cover := ui.Box(c.Key(item.key)).Absolute().Top(0).Left(0).Size(size, size).
				Radius(14).Label(label).Background(white.Alpha(0.12)).
				Shadow(0, 20, 44, -10, ui.RGBA(0, 0, 0, 0.55)).
				Transition(ui.ElementTransition{Colors: true, Animation: &fmCoverEnter, Enter: &ui.Motion{}})
			cover.AnimatePose(pose, animation)
			cover.Children(func() { a.coverFill(c, item.track.Album.PicURL, 600, 14) })
		}
		if next >= 0 && next != current {
			draw(fmCoverItem{key: fmCoverKey{queue[next].ID, next}, track: queue[next]},
				ui.Pose{X: 34, Scale: float32(size-32) / size, Rotate: 4}, fmCoverMove, "下一首封面："+queue[next].Name)
		}
		pose := ui.Pose{}
		if !a.pl.Playing() {
			pose.Scale = 0.94
		}
		draw(state.current, pose, fmCoverMove, "当前封面："+tr.Name)
		for i := len(state.leaving) - 1; i >= 0; i-- {
			draw(state.leaving[i], ui.Pose{X: -42, Y: 3, Scale: 0.96, Rotate: -2, Fade: 1}, fmCoverLeave, "")
		}
	})
}

// heartCard starts heart mode: songs like those the listener likes.
func (a *app) heartCard(c *ui.Context, p palette) {
	on := a.pl.Radio() == heartRadio
	ui.Row(c).Padding(22, 26).Gap(20).Radius(20).AlignItems(ui.Center).Background(p.hover).Children(func() {
		ui.Box(c).Size(56, 56).Radius(14).Center().Shrink(0).
			LinearGradient(ui.LinearGradient{Angle: 135, Oklab: true, From: ui.Oklch(0.72, 0.17, 10), To: ui.Oklch(0.58, 0.22, 350)}).
			TextColor(white).Children(func() { ui.Icon(c, iconHeartFill).Size(26, 26) })
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, "从你喜欢的音乐出发").FontSize(15).FontWeight(700)
			ui.Text(c, "根据你红心过的歌曲，智能推荐风格相似的音乐，与你的喜欢穿插播放").FontSize(12.5).TextColor(p.muted)
		})
		if on {
			if pillButton(c.Key("exit-heart"), iconX, "退出心动模式", p.press.Over(p.bg), p.text).Clicked() {
				a.exitRadio()
			}
		}
		label := "开启心动模式"
		switch {
		case a.profile == nil:
			label = "登录后开启"
		case on:
			label = "正在播放"
		case a.radioStarting == heartRadio:
			label = "正在开启…"
		}
		if pillButton(c.Key("heart"), iconHeartFill, label, p.accent, p.onAccent).Clicked() {
			a.startHeartMode()
		}
	})
}
