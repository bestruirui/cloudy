package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// copyText puts text on the clipboard; tests replace it.
var copyText = func(text string) error { return mygo.Clipboard.Write(transfer.TextData(text)) }

// songActions lists what can be done with tr, in groups: copying its title,
// its title with its artists, or the link to its page, opening its artists'
// or album's, and, signed in, adding it to a playlist or taking it out of
// pid, the playlist it is listed in, if set.
func (a *app) songActions(tr *netease.Track, pid int64) [][]menuAction {
	if tr.ProgramID != 0 {
		return a.programActions(tr)
	}
	groups := [][]menuAction{
		{
			{label: "复制歌名", icon: iconCopy, do: func() { copyText(tr.Name) }},
			{label: "复制歌名和歌手", icon: iconCopy, do: func() { copyText(tr.Name + " - " + tr.ArtistNames()) }},
		},
		{{label: "复制链接", icon: iconLink, do: func() { copyText(tr.URL()) }}},
	}
	// Its artists' and album's pages, those NetEase has.
	var pages []menuAction
	for _, ar := range tr.Artists {
		if ar.ID != 0 {
			pages = append(pages, menuAction{label: "查看歌手：" + ar.Name, title: ar.Name, note: "歌手", icon: iconUser,
				do: func() { a.openArtist(ar.ID) }})
		}
	}
	if tr.Album.ID != 0 {
		pages = append(pages, menuAction{label: "查看专辑：" + tr.Album.Name, title: tr.Album.Name, note: "专辑", icon: iconAlbum,
			do: func() { a.openPage(route{page: pageAlbum, id: tr.Album.ID}) }})
	}
	if len(pages) > 0 {
		groups = append(groups, pages)
	}
	if lists := a.playlistActions(tr, pid); len(lists) > 0 {
		groups = append(groups, lists)
	}
	return groups
}

// programActions lists what can be done with an episode: copying its title
// or the link to its page, or opening its podcast's.
func (a *app) programActions(tr *netease.Track) [][]menuAction {
	return [][]menuAction{
		{
			{label: "复制节目名", icon: iconCopy, do: func() { copyText(tr.Name) }},
			{label: "复制链接", icon: iconLink, do: func() { copyText(tr.URL()) }},
		},
		{{label: "查看播客", icon: iconPodcast, do: func() { a.openPage(route{page: pagePodcast, id: tr.PodcastID}) }}},
	}
}

// songHead heads the menu of tr with what it is about.
func (a *app) songHead(tr *netease.Track) func(c *ui.Context, ink paneInk) {
	return func(c *ui.Context, ink paneInk) {
		ui.Row(c).Height(58).Padding(0, 8, 0, 8).Gap(11).AlignItems(ui.Center).Children(func() {
			a.cover(c, tr.Album.PicURL, 96, 40, 8).Shrink(0)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, tr.Name).FontSize(13.5).FontWeight(600).SingleLine()
				ui.Text(c, tr.ArtistNames()).FontSize(12).TextColor(ink.muted).SingleLine()
			})
		})
	}
}

// songMenu is three dots that open the menu of tr, over Now Playing.
func (a *app) songMenu(c *ui.Context, tr *netease.Track, size float32, color ui.Color) ui.Element {
	b := iconButton(c, iconMore, size, color, "更多")
	if b.Clicked() {
		a.menuBelow(b, true, a.songActions(tr, 0), a.songHead(tr))
	}
	return b
}

// songContext opens the menu of tr, listed in pid if set, on a right click
// on el, what shows it.
func (a *app) songContext(el ui.Element, tr *netease.Track, pid int64) {
	a.contextMenu(el, a.nowPlaying, func() [][]menuAction { return a.songActions(tr, pid) }, a.songHead(tr))
}
