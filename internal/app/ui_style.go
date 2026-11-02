package app

import (
	"errors"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Roaming a style plays on through songs of one kind, as the app's radios do,
// as "style:<label>". NetEase's own scene modes aren't open beyond a few, and
// need the listener signed in: this goes by the public playlists of the style,
// best played first, which works for anyone, though not to their taste.

// A roamStyle is a style to roam: what it is called, what shows on its tile,
// and what finds its playlists.
type roamStyle struct {
	label string
	icon  *ui.SVG
	glyph string // in place of an icon, for a language
	words string
}

var roamGroups = []struct {
	title  string
	styles []roamStyle
}{
	{"心情", []roamStyle{
		{label: "伤感", icon: iconHeartCrack, words: "伤感"},
		{label: "放松", icon: iconFlower, words: "放松"},
		{label: "欢快", icon: iconParty, words: "欢快"},
		{label: "抒情", icon: iconFeather2, words: "抒情"},
		{label: "治愈", icon: iconSprout, words: "治愈"},
		{label: "情歌", icon: iconHeart2, words: "情歌"},
		{label: "励志", icon: iconFlame, words: "励志"},
		{label: "经典", icon: iconTape, words: "经典 老歌"},
	}},
	{"场景", []roamStyle{
		{label: "运动", icon: iconDumbbell, words: "运动 跑步"},
		{label: "助眠", icon: iconMoonStar, words: "助眠"},
		{label: "专注", icon: iconBook, words: "专注 学习"},
		{label: "下雨天", icon: iconUmbrella, words: "下雨天"},
		{label: "打游戏", icon: iconGamepad, words: "打游戏"},
		{label: "出行", icon: iconCar, words: "出行 自驾"},
		{label: "洗澡", icon: iconBath, words: "洗澡"},
		{label: "咖啡馆", icon: iconCoffee, words: "咖啡馆"},
		{label: "舞蹈", icon: iconSteps, words: "舞蹈"},
		{label: "影视", icon: iconClapper, words: "影视金曲"},
	}},
	{"语种", []roamStyle{
		{label: "华语", glyph: "中", words: "华语"},
		{label: "欧美", glyph: "En", words: "欧美"},
		{label: "粤语", glyph: "粤", words: "粤语"},
		{label: "日语", glyph: "あ", words: "日语"},
		{label: "K-Pop", glyph: "한", words: "K-Pop"},
		{label: "法语", glyph: "Fr", words: "法语"},
		{label: "全球", icon: iconGlobe, words: "世界音乐"},
	}},
	{"曲风", []roamStyle{
		{label: "R&B", icon: iconMicVocal, words: "R&B"},
		{label: "说唱", icon: iconMic, words: "说唱"},
		{label: "电音", icon: iconWave, words: "电音"},
		{label: "摇滚", icon: iconZap, words: "摇滚"},
		{label: "民谣", icon: iconGuitar, words: "民谣"},
		{label: "国风", icon: iconFan, words: "国风"},
		{label: "轻音乐", icon: iconLeaf, words: "轻音乐"},
		{label: "古典", icon: iconPiano, words: "古典"},
		{label: "爵士", icon: iconWine, words: "爵士"},
		{label: "蓝调", icon: iconCloudMoon, words: "蓝调"},
		{label: "慢摇 DJ", icon: iconDisc, words: "慢摇 DJ"},
		{label: "二次元", icon: iconWand, words: "二次元"},
		{label: "拉丁", icon: iconSun, words: "拉丁"},
		{label: "放克", icon: iconRadioOld, words: "放克 Funk"},
		{label: "乡村乐", icon: iconWheat, words: "乡村"},
		{label: "音乐剧", icon: iconDrama, words: "音乐剧"},
		{label: "宝藏原创", icon: iconGem, words: "宝藏 原创"},
	}},
}

const stylePrefix = "style:"

func styleRadio(label string) string { return stylePrefix + label }

// isRoam reports whether a radio plays a style, not the FM.
func isRoam(name string) bool { return strings.HasPrefix(name, stylePrefix) }

// styleWords returns what finds the playlists of the style of a radio.
func styleWords(label string) string {
	for _, g := range roamGroups {
		for _, s := range g.styles {
			if s.label == label {
				return s.words
			}
		}
	}
	return label
}

// How a style goes on: from one playlist at a time, a few songs at a time, of
// the best played it finds.
const (
	roamPlaylists = 12
	roamBatch     = 8
)

// roamer deals the songs of styles out of their playlists, on the goroutines
// radios load on.
type roamer struct {
	mu      sync.Mutex
	pools   map[string][]netease.Playlist // the playlists found for a style
	dealing map[string][]netease.Track    // what is left of the one it is on
	played  map[int64]bool                // songs dealt, none twice
}

// next returns a few songs of a style that haven't played.
func (r *roamer) next(label string) ([]netease.Track, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pools == nil {
		r.pools, r.dealing, r.played = map[string][]netease.Playlist{}, map[string][]netease.Track{}, map[int64]bool{}
	}
	for tries := 0; tries < 4; tries++ {
		var out []netease.Track
		left := r.dealing[label]
		for len(left) > 0 && len(out) < roamBatch {
			tr := left[0]
			left = left[1:]
			if !r.played[tr.ID] && tr.Playable() {
				r.played[tr.ID] = true
				out = append(out, tr)
			}
		}
		r.dealing[label] = left
		if len(out) > 0 {
			return out, nil
		}
		if err := r.refill(label); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("style: no songs left")
}

// refill opens another of the style's playlists, shuffled: more often one of
// the best played.
func (r *roamer) refill(label string) error {
	pool := r.pools[label]
	if len(pool) == 0 {
		res, err := netease.SearchPage(styleWords(label), netease.SearchPlaylists, roamPlaylists, 0)
		if err != nil {
			return err
		}
		if pool = res.Playlists; len(pool) == 0 {
			return errors.New("style: no playlists")
		}
	}
	at := min(rand.IntN(len(pool)), rand.IntN(len(pool)))
	pl := pool[at]
	r.pools[label] = append(pool[:at:at], pool[at+1:]...)
	detail, err := netease.PlaylistDetail(pl.ID)
	if err != nil {
		return err
	}
	songs := append([]netease.Track(nil), detail.Tracks...)
	rand.Shuffle(len(songs), func(i, j int) { songs[i], songs[j] = songs[j], songs[i] })
	r.dealing[label] = songs
	return nil
}

// playStyle roams a style, or goes on with it.
func (a *app) playStyle(label string) {
	name := styleRadio(label)
	if a.pl.Radio() == name {
		a.nowPlaying = true
		return
	}
	a.startRadio(name, "这个风格暂时无法播放，请稍后再试")
}

// roamSection lists the styles, each a tile that starts it, in groups.
func (a *app) roamSection(c *ui.Context) {
	p := a.pal
	a.section(c, "风格漫游")
	ui.Text(c, "选一种风格，一直播放这一类的歌，不需要登录").FontSize(12.5).TextColor(p.muted).Margin(-8, 0, 16, 0)
	for _, g := range roamGroups {
		ui.Text(c, g.title).FontSize(12).FontWeight(600).TextColor(p.faint).Margin(12, 2, 8, 2)
		ui.Grid(c.Key(g.title)).Columns(a.columns(c, 88)).GapX(10).GapY(10).Children(func() {
			for i := range g.styles {
				a.styleTile(c.Key(g.styles[i].label), &g.styles[i])
			}
		})
	}
}

// styleTile is a style: lit while it plays.
func (a *app) styleTile(c *ui.Context, s *roamStyle) {
	p := a.pal
	name := styleRadio(s.label)
	on, starting := a.pl.Radio() == name, a.radioStarting == name
	tile := ui.ButtonBase(c).Column().Height(82).Gap(7).Radius(14).AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).Label(s.label)
	bg, ink := p.hover, p.text
	switch {
	case on:
		bg, ink = p.accent.Alpha(0.16), p.accent
	case tile.Hovered():
		bg = p.press
	}
	tile.Background(tile.AnimateColor("bg", bg, ui.Animation{Duration: 140 * time.Millisecond}))
	lift := ui.Pose{}
	if tile.Hovered() {
		lift.Y = -2
	}
	if tile.Pressed() {
		lift = ui.Pose{Scale: 0.97}
	}
	tile.AnimatePose(lift, ui.Snappy)
	tile.Children(func() {
		switch {
		case on:
			a.nowPlayingBars(c)
		case s.icon != nil:
			ui.Icon(c, s.icon).Size(22, 22).TextColor(ink)
		default:
			ui.Text(c, s.glyph).FontSize(19).FontWeight(800).TextColor(ink).LineHeight(1.15)
		}
		label := s.label
		if starting {
			label = "正在调频…"
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(500).TextColor(ink).SingleLine()
	})
	if tile.Clicked() {
		a.playStyle(s.label)
	}
}
