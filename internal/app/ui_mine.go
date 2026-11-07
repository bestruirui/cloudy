package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// The listener's own page is where they keep what they keep: the playlists
// they made and saved, the albums they saved, the artists they follow and
// the podcasts they subscribe to. Each kind shows a row, and the rest as
// it is opened, for there may be hundreds; a search over the page finds in
// all of them at once, the albums by their artists too. The sidebar lists
// the first few of its groups, and opens the page for the rest.

// mineView is the search on the listener's page and the kinds opened on
// it, kept while it is away.
type mineView struct {
	query string
	box   ui.Handle
	open  map[string]bool // by the key of the kind
	// only is the kind the page is narrowed to, of mineFilters: 0 for all.
	only int
}

// mineFilters are the kinds the page may be narrowed to, the podcasts
// showing only among all.
var mineFilters = []string{"全部", "歌单", "专辑", "歌手"}

const (
	onlyAll = iota
	onlyPlaylists
	onlyAlbums
	onlyArtists
)

// shown reports whether the page shows kind, as it is narrowed.
func (v *mineView) shown(kind string) bool {
	switch v.only {
	case onlyPlaylists:
		return kind == mineMade || kind == mineSaved
	case onlyAlbums:
		return kind == mineAlbums
	case onlyArtists:
		return kind == mineArtists
	}
	return true
}

// The kinds the page lists, by the keys their opening is kept under.
const (
	mineMade     = "made"
	mineSaved    = "saved"
	mineAlbums   = "albums"
	minePodcasts = "podcasts"
	mineArtists  = "artists"
)

// playlistSorts are the orders the page may list playlists in.
var playlistSorts = []struct{ id, label string }{
	{"", "默认顺序"},
	{"name", "按名称"},
	{"songs", "按歌曲数"},
}

// sidebarShown is how many of each group the sidebar lists, unless it is
// set to list all of them.
const sidebarShown = 8

// openMine opens the listener's page with kind opened, as from the sidebar
// for the rest of a group.
func (a *app) openMine(kind string) {
	if a.profile == nil {
		return
	}
	v := &a.mineView
	if v.open == nil {
		v.open = map[string]bool{}
	}
	v.open[kind], v.query = true, ""
	a.open(route{page: pageUser, id: a.profile.UserID})
}

// sideShown is how many of n rows the sidebar lists.
func (a *app) sideShown(n int) int {
	if a.settings.SidebarAll {
		return n
	}
	return min(n, sidebarShown)
}

// sideMore is the sidebar's row under a group it lists only the first few
// of, to the listener's page with them all.
func (a *app) sideMore(c *ui.Context, n int, kind string) {
	p := a.pal
	label := fmt.Sprintf("查看全部 %d 个", n)
	if kind == mineAlbums {
		label = fmt.Sprintf("查看全部 %d 张", n)
	}
	b := ui.ButtonBase(c.Key("more")).Height(30).PaddingX(10).Gap(6).Radius(8).AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(label)
	color := p.faint
	if b.Hovered() {
		b.Background(p.hover)
		color = p.muted
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(color).Grow(1)
		ui.Icon(c, iconForward).Size(11, 11).TextColor(color)
	})
	if b.Clicked() {
		a.openMine(kind)
	}
}

// mineQuery is what a search matches, in lower case.
func mineQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// filterPlaylists returns the playlists whose name or creator has q in it.
func filterPlaylists(lists []netease.Playlist, q string) []netease.Playlist {
	if q == "" {
		return lists
	}
	var out []netease.Playlist
	for _, pl := range lists {
		if strings.Contains(strings.ToLower(pl.Name), q) || strings.Contains(strings.ToLower(pl.Creator.Nickname), q) {
			out = append(out, pl)
		}
	}
	return out
}

// sortPlaylists returns the playlists in the order by, the liked songs
// first whatever it is, as NetEase lists them.
func sortPlaylists(lists []netease.Playlist, by string) []netease.Playlist {
	if by == "" {
		return lists
	}
	out := slices.Clone(lists)
	slices.SortStableFunc(out, func(x, y netease.Playlist) int {
		if lx, ly := x.SpecialType == 5, y.SpecialType == 5; lx != ly {
			if lx {
				return -1
			}
			return 1
		}
		if by == "songs" {
			return y.TrackCount - x.TrackCount
		}
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return out
}

// filterAlbums returns the albums q is in: those of an artist it is in
// first, as one searching an artist looks for theirs, then those by their
// own name, each in the order they come.
func filterAlbums(albums []netease.AlbumProfile, q string) []netease.AlbumProfile {
	if q == "" {
		return albums
	}
	var byArtist, byName []netease.AlbumProfile
	for _, al := range albums {
		switch {
		case slices.ContainsFunc(al.Artists, func(ar netease.Artist) bool { return strings.Contains(strings.ToLower(ar.Name), q) }):
			byArtist = append(byArtist, al)
		case strings.Contains(strings.ToLower(al.Name), q):
			byName = append(byName, al)
		}
	}
	return append(byArtist, byName...)
}

// filterArtists returns the artists whose name, or a name they go by, has
// q in it.
func filterArtists(artists []netease.ArtistProfile, q string) []netease.ArtistProfile {
	if q == "" {
		return artists
	}
	var out []netease.ArtistProfile
	for _, ar := range artists {
		names := append([]string{ar.Name, ar.Trans}, ar.Alias...)
		if slices.ContainsFunc(names, func(n string) bool { return n != "" && strings.Contains(strings.ToLower(n), q) }) {
			out = append(out, ar)
		}
	}
	return out
}

// filterPodcasts returns the podcasts whose name or host has q in it.
func filterPodcasts(pods []netease.Podcast, q string) []netease.Podcast {
	if q == "" {
		return pods
	}
	var out []netease.Podcast
	for _, pod := range pods {
		if strings.Contains(strings.ToLower(pod.Name), q) || strings.Contains(strings.ToLower(pod.DJ.Nickname), q) {
			out = append(out, pod)
		}
	}
	return out
}

// mineSearch is the search over the listener's page, under who they are.
func (a *app) mineSearch(c *ui.Context) {
	v := &a.mineView
	ui.Row(c).AlignItems(ui.Center).Gap(14).Margin(28, 0, 0, 0).Children(func() {
		a.mineField(c)
		if a.segmented(c.Key("mine-only"), &v.only, mineFilters...) {
			v.box.Focus(c)
		}
	})
}

// mineField is the search field over the listener's page, saying what it
// finds in the kind the page is narrowed to.
func (a *app) mineField(c *ui.Context) {
	p := a.pal
	v := &a.mineView
	field := ui.Row(c).Width(300).Height(34).PaddingX(12).Gap(7).Radius(999).AlignItems(ui.Center)
	if c.Focused(v.box) {
		field.Background(p.bg).Shadow(0, 0, 0, 3, p.accent.Alpha(0.25)).Border(1, p.accent.Alpha(0.6))
	} else {
		field.Background(p.hover).Border(1, p.hairline)
	}
	hint := [...]string{"搜索歌单、专辑、歌手和播客", "搜索歌单名或创建者", "搜索专辑名或歌手名", "搜索歌手名"}[v.only]
	field.Children(func() {
		ui.Icon(c, iconSearch).Size(13, 13).TextColor(p.muted)
		ui.TextInputBase(c.Key("mine-filter"), &v.query).Bind(&v.box).Grow(1).MinWidth(0).
			Placeholder(hint).FontSize(12.5).Label(hint)
		if v.query != "" && iconButton(c.Key("clear"), iconX, 11, p.muted, "清除搜索").Size(20, 20).Clicked() {
			v.query = ""
		}
	})
}

// foldHead is the title of a kind on the page, with how many there are, a
// control of their order if order is set, and the button that shows those
// past the first row, or hides them again. It reports whether they show:
// all do while the page is searched.
func (a *app) foldHead(c *ui.Context, kind, title string, n, perRow int, order func(c *ui.Context)) bool {
	p := a.pal
	v := &a.mineView
	// Searched or narrowed to it, a kind shows all it has.
	searching := mineQuery(v.query) != "" || v.only != onlyAll
	open := searching || v.open[kind]
	ui.Row(c.Key("head-"+kind)).AlignItems(ui.Center).Gap(10).Margin(34, 0, 14, 0).Children(func() {
		ui.Text(c, title).FontSize(19).FontWeight(700).LetterSpacing(-0.3)
		ui.Text(c, fmt.Sprint(n)).FontSize(13).FontWeight(600).TextColor(p.faint).Margin(3, 0, 0, 0)
		ui.Box(c).Grow(1)
		if order != nil && n > 1 {
			order(c)
		}
		if searching || n <= perRow {
			return
		}
		label := fmt.Sprintf("展开全部 %d", n)
		if open {
			label = "收起"
		}
		b := ui.ButtonBase(c.Key("fold")).Height(28).PaddingX(12).Gap(6).Radius(999).AlignItems(ui.Center).FocusRing(false).
			Role(ui.RoleDisclosure).Label(label)
		b.Expanded(open)
		bg := p.hover
		if b.Hovered() {
			bg = p.press
		}
		b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
		b.Children(func() {
			ui.Text(c, label).FontSize(12.5).SingleLine()
			turn := float32(0)
			if open {
				turn = 180
			}
			ui.Icon(c, iconDown).Size(12, 12).TextColor(p.muted).Rotate(b.AnimateTo("turn", turn, motionFast))
		})
		if b.Clicked() {
			if v.open == nil {
				v.open = map[string]bool{}
			}
			v.open[kind] = !open
		}
	})
	return open
}

// foldGrid lays n things out as grid does, the first row of them, and the
// rest under it while open, opening and closing as the sidebar's groups do.
func (a *app) foldGrid(c *ui.Context, kind string, open bool, n, perRow int, gapY float32, grid func(c *ui.Context) ui.Element, item func(c *ui.Context, i int)) {
	first := min(n, perRow)
	grid(c.Key(kind + "-first")).Children(func() {
		for i := range first {
			item(c, i)
		}
	})
	if n <= first || !open {
		return
	}
	rest := ui.Column(c.Key(kind+"-rest")).Margin(gapY, 0, 0, 0)
	if a.motion > motionOff {
		rest.Transition(ui.ElementTransition{Animation: &motionPane, Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}})
	}
	rest.Children(func() {
		grid(c).Children(func() {
			for i := first; i < n; i++ {
				item(c, i)
			}
		})
	})
}

// playlistOrder is the control of how the page lists playlists.
func (a *app) playlistOrder(c *ui.Context) {
	label := playlistSorts[0].label
	var acts []menuAction
	for _, s := range playlistSorts {
		id := s.id
		if id == a.settings.PlaylistSort {
			label = s.label
		}
		acts = append(acts, menuAction{label: s.label, checked: id == a.settings.PlaylistSort, do: func() {
			a.settings.PlaylistSort = id
			a.changed()
		}})
	}
	a.orderChip(c, label, [][]menuAction{acts})
}

// albumOrder is the control of how the page lists albums, which the
// sidebar keeps to too.
func (a *app) albumOrder(c *ui.Context) {
	label := albumSorts[0].label
	for _, s := range albumSorts {
		if s.id == a.albumSort() {
			label = s.label
		}
	}
	a.orderChip(c, label, a.albumSortMenu())
}

// orderChip is the button of an order, which opens the menu of the others.
func (a *app) orderChip(c *ui.Context, label string, menu [][]menuAction) {
	p := a.pal
	b := ui.ButtonBase(c.Key("order")).Height(28).PaddingX(10).Gap(5).Radius(999).AlignItems(ui.Center).FocusRing(false).Label("排序：" + label)
	color := p.muted
	if b.Hovered() {
		b.Background(p.hover)
		color = p.text
	}
	b.Children(func() {
		ui.Icon(c, iconUpDown).Size(12, 12).TextColor(color)
		ui.Text(c, label).FontSize(12.5).TextColor(color).SingleLine()
	})
	if b.Clicked() {
		a.menuBelow(b, false, menu, nil)
	}
}

// minePage is the part of the listener's own page under who they are.
func (a *app) minePage(c *ui.Context, d *userData) {
	q := mineQuery(a.mineView.query)
	a.mineSearch(c)
	cardCols := a.columns(c, 168)
	cardGrid := func(c *ui.Context) ui.Element {
		return ui.Grid(c).Columns(cardCols).GapX(20).GapY(28)
	}
	found := 0
	// Searched, a kind with nothing found is left out.
	shows := func(n int) bool {
		found += n
		return q == "" || n > 0
	}

	// The playlists come a moment after the page; had the sidebar's not
	// come, those the page loaded show.
	made, saved := a.mine, a.saved
	if made == nil && saved == nil && d.loading == 0 {
		made = d.playlists
	}
	if made == nil && saved == nil && d.loading > 0 && a.mineView.shown(mineMade) {
		a.section(c, "创建的歌单")
		a.playlistCardsSkeleton(c, a.pageShimmer(c), cardCols, 168, false)
	} else {
		for _, kind := range []string{mineMade, mineSaved} {
			mine := kind == mineMade
			all, title := made, "创建的歌单"
			if !mine {
				all, title = saved, "收藏的歌单"
			}
			lists := sortPlaylists(filterPlaylists(all, q), a.settings.PlaylistSort)
			if !a.mineView.shown(kind) || !shows(len(lists)) || !mine && len(all) == 0 {
				continue
			}
			open := a.foldHead(c, kind, title, len(lists), cardCols, a.playlistOrder)
			if len(all) == 0 {
				a.empty(c, "还没有创建歌单")
				continue
			}
			a.foldGrid(c, kind, open, len(lists), cardCols, 28, cardGrid, func(c *ui.Context, i int) {
				a.minePlaylistCard(c, &lists[i], mine)
			})
		}
	}

	albums := filterAlbums(a.sortedAlbums(), q)
	if a.mineView.shown(mineAlbums) && len(a.myAlbums) > 0 && shows(len(albums)) {
		open := a.foldHead(c, mineAlbums, "收藏的专辑", len(albums), cardCols, a.albumOrder)
		a.foldGrid(c, mineAlbums, open, len(albums), cardCols, 28, cardGrid, func(c *ui.Context, i int) {
			al := &albums[i]
			card := a.albumCard(c.Key(al.ID), al)
			a.contextMenu(card, false, func() [][]menuAction {
				return [][]menuAction{{{label: "取消收藏", do: func() { a.toggleAlbum(*al) }}}}
			}, nil)
		})
	}

	if d.artistsShown && a.mineView.shown(mineArtists) {
		artists := filterArtists(d.artists, q)
		if shows(len(artists)) {
			cols := a.columns(c, 132)
			open := a.foldHead(c, mineArtists, "关注的歌手", len(artists), cols, nil)
			if len(d.artists) == 0 {
				a.empty(c, "还没有关注歌手")
			}
			a.foldGrid(c, mineArtists, open, len(artists), cols, 28, func(c *ui.Context) ui.Element {
				return ui.Grid(c).Columns(cols).GapX(20).GapY(28)
			}, func(c *ui.Context, i int) {
				a.artistCard(c.Key(artists[i].ID), &artists[i])
			})
		}
	}

	if d.podcastsShown && a.mineView.shown(minePodcasts) {
		pods := filterPodcasts(d.podcasts, q)
		if shows(len(pods)) {
			cols := a.columns(c, 340)
			open := a.foldHead(c, minePodcasts, "订阅的播客", len(pods), cols, nil)
			if len(d.podcasts) == 0 {
				a.empty(c, "还没有订阅播客")
			}
			a.foldGrid(c, minePodcasts, open, len(pods), cols, 4, func(c *ui.Context) ui.Element {
				return ui.Grid(c).Columns(cols).GapX(8).GapY(4).Margin(0, -12)
			}, func(c *ui.Context, i int) {
				a.podcastRow(c.Key(pods[i].ID), &pods[i])
			})
		}
	}

	if q != "" && found == 0 {
		what := [...]string{"歌单、专辑、歌手或播客", "歌单", "专辑", "歌手"}[a.mineView.only]
		a.empty(c, fmt.Sprintf("没有找到包含「%s」的%s", strings.TrimSpace(a.mineView.query), what))
	}
}

// minePlaylistCard is a playlist on the listener's page, with the menu the
// sidebar has for it.
func (a *app) minePlaylistCard(c *ui.Context, pl *netease.Playlist, mine bool) {
	sub := fmt.Sprintf("%d 首", pl.TrackCount)
	if !mine && pl.Creator.Nickname != "" {
		sub = pl.Creator.Nickname + " · " + sub
	}
	card, clicked := a.coverCardOf(c.Key(pl.ID), pl.Name, pl.Cover(), pl.PlayCount, sub, func() { a.loadPlaylist(pl.ID, true) })
	switch {
	case mine && pl.SpecialType != 5:
		a.contextMenu(card, false, func() [][]menuAction { return a.playlistMenu(pl) }, nil)
	case !mine:
		a.contextMenu(card, false, func() [][]menuAction {
			return [][]menuAction{{{label: "取消收藏", do: func() { a.toggleSaved(*pl) }}}}
		}, nil)
	}
	if clicked {
		a.open(route{page: pagePlaylist, id: pl.ID})
	}
}
