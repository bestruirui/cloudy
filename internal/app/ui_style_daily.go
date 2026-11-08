package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The day's songs in the styles the listener picked, the official client's
// 风格推荐: a card beside 私人雷达 on the home page, opening a page of the
// songs, whose styles a dialog changes.

// styleState is the day's style songs, and the dialog that picks their
// styles.
type styleState struct {
	songs   netease.StyleSongs
	loaded  bool
	loading bool
	err     string

	// The dialog: the categories of styles, the one shown, and the tags
	// picked, all of one category.
	open        bool
	cats        []netease.StyleCategory
	catsLoading bool
	catsErr     string
	tab         int64
	cat         int64
	picked      []int64
	saving      bool
	saveErr     string
}

var (
	dailyStyleSongs = netease.DailyStyleSongs
	styleCategories = netease.StyleCategories
	saveStyleTags   = netease.SaveStyleTags
)

// loadStyle loads the day's style songs for whoever is signed in.
func (a *app) loadStyle() {
	if a.profile == nil {
		a.style = styleState{}
		return
	}
	if a.style.loading {
		return
	}
	a.style.loading, a.style.err = true, ""
	gen := a.accountGen
	go func() {
		songs, err := dailyStyleSongs()
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s := &a.style
			s.loading = false
			if err != nil {
				s.err = "风格推荐加载失败"
				return
			}
			s.songs, s.loaded = songs, true
		})
	}()
}

// styleNames names the styles picked, as "R&B · 欧美流行".
func styleNames(picked netease.StyleCategory) string {
	names := make([]string, 0, len(picked.Tags))
	for _, t := range picked.Tags {
		names = append(names, t.Name)
	}
	return strings.Join(names, " · ")
}

// styleCover is the cover of the style songs: their first song's.
func styleCover(songs []netease.Track) string {
	for _, tr := range songs {
		if tr.Album.PicURL != "" {
			return tr.Album.PicURL
		}
	}
	return ""
}

// styleCardShown reports whether the home page has the style songs' card.
func (a *app) styleCardShown() bool {
	return a.profile != nil && a.style.loaded && len(a.style.songs.Songs) > 0
}

// recommendedGrid is the day's playlists picked for the listener, with the
// style songs' card just after 私人雷达, or first.
func (a *app) recommendedGrid(c *ui.Context) {
	cols := a.columns(c, 168)
	style := a.styleCardShown()
	n := cols * 2
	if style {
		n--
	}
	recs := a.recPlaylists[:min(len(a.recPlaylists), n)]
	after := -1
	if style {
		after = slices.IndexFunc(recs, func(pl netease.Playlist) bool { return pl.Name == "私人雷达" })
	}
	ui.Grid(c).Columns(cols).GapX(20).GapY(28).Children(func() {
		if style && after < 0 {
			a.styleCard(c.Key("style-songs"))
		}
		for i := range recs {
			a.playlistCard(c.Key(recs[i].ID), &recs[i])
			if style && i == after {
				a.styleCard(c.Key("style-songs"))
			}
		}
	})
}

// styleCard is the style songs' card: it opens their page.
func (a *app) styleCard(c *ui.Context) {
	s := &a.style
	play := func() { a.playList(s.songs.Songs, false) }
	if a.coverCard(c, "风格推荐", styleCover(s.songs.Songs), 0, styleNames(s.songs.Picked), play) {
		a.open(route{page: pageStyle})
	}
}

// stylePage is the day's songs in the styles picked, with a button that
// changes those.
func (a *app) stylePage(c *ui.Context) {
	s := &a.style
	if a.profile == nil {
		a.centered(c, "登录后才能使用风格推荐", nil)
		return
	}
	loading := !s.loaded
	if loading && !s.loading {
		err := s.err
		if err == "" {
			err = "暂时没有风格推荐"
		}
		a.centered(c, err, a.loadStyle)
		return
	}
	songs := s.songs.Songs
	sh := a.pageShimmer(c)
	header := func(c *ui.Context) {
		p := a.pal
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
			a.headCover(c, styleCover(songs))
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
				ui.Text(c, "每日推荐").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, "风格推荐").FontSize(30).FontWeight(800).LetterSpacing(-0.6)
				if loading {
					sh.text(c, 13, 1.3).Width(240)
					return
				}
				ui.Text(c, fmt.Sprintf("%d 首 · 按你选的风格挑选，每天更新", len(songs))).FontSize(13).TextColor(p.muted)
				ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
						a.playList(songs, false)
					}
					if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
						a.playList(songs, true)
					}
					label := styleNames(s.songs.Picked)
					if label == "" {
						label = "选择风格"
					}
					if pillButton(c.Key("styles"), iconWand, label, p.hover.Over(p.bg), p.text).Label("选择风格：" + label).Clicked() {
						a.openStylePicker()
					}
					a.batchEntry(c)
				})
			})
		})
	}
	if loading {
		a.trackSkeleton(c, sh, true, header)
		return
	}
	a.trackRows(c, songs, true, header)
}

// openStylePicker opens the dialog on the styles picked, loading what can
// be picked if it has not.
func (a *app) openStylePicker() {
	s := &a.style
	s.open, s.saveErr = true, ""
	s.cat = s.songs.Picked.ID
	s.picked = s.picked[:0]
	for _, t := range s.songs.Picked.Tags {
		s.picked = append(s.picked, t.ID)
	}
	s.tab = s.cat
	if len(s.cats) > 0 || s.catsLoading {
		a.styleTabFallback()
		return
	}
	s.catsLoading, s.catsErr = true, ""
	gen := a.accountGen
	go func() {
		cats, err := styleCategories()
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s.catsLoading = false
			if err != nil || len(cats) == 0 {
				s.catsErr = "风格列表加载失败"
				return
			}
			s.cats = cats
			a.styleTabFallback()
		})
	}()
}

// styleTabFallback shows the first category if the one picked in isn't
// one there is.
func (a *app) styleTabFallback() {
	s := &a.style
	if len(s.cats) > 0 && !slices.ContainsFunc(s.cats, func(cat netease.StyleCategory) bool { return cat.ID == s.tab }) {
		s.tab = s.cats[0].ID
	}
}

// pickStyle picks tag of category cat, or unpicks it. Tags are picked in
// one category: one of another starts the picking again.
func (a *app) pickStyle(cat, tag int64) {
	s := &a.style
	if s.cat != cat {
		s.cat, s.picked = cat, []int64{tag}
		return
	}
	if i := slices.Index(s.picked, tag); i >= 0 {
		s.picked = slices.Delete(s.picked, i, i+1)
		return
	}
	s.picked = append(s.picked, tag)
}

// saveStyles keeps the styles picked with the account, and loads the songs
// in them.
func (a *app) saveStyles() {
	s := &a.style
	if s.saving || len(s.picked) == 0 {
		return
	}
	s.saving, s.saveErr = true, ""
	cat, tags := s.cat, slices.Clone(s.picked)
	gen := a.accountGen
	go func() {
		err := saveStyleTags(cat, tags)
		var songs netease.StyleSongs
		if err == nil {
			songs, err = dailyStyleSongs()
		}
		a.post(func() {
			if gen != a.accountGen {
				return
			}
			s.saving = false
			if err != nil {
				s.saveErr = "保存失败，请稍后再试"
				return
			}
			s.songs, s.loaded, s.open = songs, true, false
		})
	}()
}

// stylePickerDialog picks the styles of the style songs: a tab for each
// category, its tags under it.
func (a *app) stylePickerDialog(c *ui.Context) {
	s := &a.style
	p := a.pal
	a.dialog(c, "style-picker", 0.35, &s.open, func(panel ui.Element) {
		a.pane(panel, 26, false)
		ink := a.inkOn(false)
		panel.Width(560).TextColor(ink.text)
		dialogHeader(c, ink, "选择风格", "同一类里可以多选，和网易云官方客户端同步", func() { s.open = false })
		ui.Column(c).Padding(0, 28).Gap(16).MinHeight(250).Children(func() {
			switch {
			case s.catsLoading:
				ui.Text(c, "正在加载…").FontSize(13).TextColor(ink.muted)
				return
			case s.catsErr != "":
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, s.catsErr).FontSize(13).TextColor(ink.muted)
					if settingButton(c.Key("retry"), ink, "重试", nil, false, false).Clicked() {
						a.openStylePicker()
					}
				})
				return
			}
			ui.Row(c.Key("tabs")).Gap(6).Children(func() {
				for _, cat := range s.cats {
					on := cat.ID == s.tab
					b := ui.ButtonBase(c.Key(cat.ID)).Height(32).PaddingX(14).Radius(999).Center().
						FocusRing(false).Label("风格分类：" + cat.Name).Selected(on)
					bg, fg := ui.Transparent, ink.muted
					switch {
					case on:
						bg, fg = ink.hover, ink.text
					case b.Hovered():
						fg = ink.text
					}
					b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
					b.Children(func() {
						label := cat.Name
						if n := a.pickedIn(cat.ID); n > 0 {
							label = fmt.Sprintf("%s %d", cat.Name, n)
						}
						ui.Text(c, label).FontSize(14).FontWeight(650).TextColor(fg).SingleLine()
					})
					if b.Clicked() {
						s.tab = cat.ID
					}
				}
			})
			for _, cat := range s.cats {
				if cat.ID != s.tab {
					continue
				}
				ui.Grid(c.Key(fmt.Sprint("tags-", cat.ID))).Columns(5).Gap(10).Children(func() {
					for _, t := range cat.Tags {
						on := s.cat == cat.ID && slices.Contains(s.picked, t.ID)
						b := ui.ButtonBase(c.Key(t.ID)).Height(34).Radius(999).Center().FocusRing(false).
							Label("风格：" + t.Name).Selected(on)
						bg, fg := ink.hover, ink.text
						switch {
						case on:
							bg, fg = p.accent, p.onAccent
						case b.Hovered():
							bg = ink.hover.Mix(ink.text, 0.08)
						}
						b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
						b.Children(func() {
							ui.Text(c, t.Name).FontSize(13).FontWeight(600).TextColor(fg).SingleLine()
						})
						if b.Clicked() {
							a.pickStyle(cat.ID, t.ID)
						}
					}
				})
			}
			if s.saveErr != "" {
				ui.Text(c, s.saveErr).FontSize(12.5).TextColor(dangerColor)
			}
		})
		dialogFooter(c, func() {
			if settingButton(c.Key("cancel"), ink, "取消", nil, false, false).Clicked() {
				s.open = false
			}
			label := "确认"
			if s.saving {
				label = "正在保存…"
			}
			if primaryButton(c.Key("ok"), label, nil, p, len(s.picked) == 0 || s.saving || len(s.cats) == 0).Clicked() {
				a.saveStyles()
			}
		})
	})
}

// pickedIn says how many tags of category cat are picked.
func (a *app) pickedIn(cat int64) int {
	if a.style.cat != cat {
		return 0
	}
	return len(a.style.picked)
}
