package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// madePlaylists are n playlists the listener made, the liked songs first.
func madePlaylists(n int) []netease.Playlist {
	lists := make([]netease.Playlist, n)
	for i := range lists {
		lists[i] = netease.Playlist{ID: int64(100 + i), Name: fmt.Sprintf("歌单 %d", i+1), TrackCount: i}
	}
	lists[0].Name, lists[0].SpecialType = "我喜欢的音乐", 5
	return lists
}

func playlistIDs(lists []netease.Playlist) []int64 {
	var out []int64
	for _, pl := range lists {
		out = append(out, pl.ID)
	}
	return out
}

// mineApp is the listener signed in on their own page, with what NetEase
// would send of it already there.
func mineApp(t *testing.T, made int) *app {
	ars, pods := followedArtists, subscribedPodcasts
	t.Cleanup(func() { followedArtists, subscribedPodcasts = ars, pods })
	followedArtists = func() ([]netease.ArtistProfile, error) {
		return []netease.ArtistProfile{{Artist: netease.Artist{ID: 7, Name: "Gamma"}}}, nil
	}
	subscribedPodcasts = func() ([]netease.Podcast, error) {
		return []netease.Podcast{{ID: 8, Name: "夜谈", DJ: netease.User{Nickname: "主播"}}}, nil
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1, Nickname: "nya_owo"}
	a.mine = madePlaylists(made)
	a.saved = []netease.Playlist{{ID: 900, Name: "别人的歌单", TrackCount: 20}}
	a.saved[0].Creator.Nickname = "云音乐"
	a.myAlbums = savedAlbums()
	a.users[1] = &userData{user: &netease.User{UserID: 1, Nickname: "nya_owo"}}
	a.users[1].artists, _ = followedArtists()
	a.users[1].podcasts, _ = subscribedPodcasts()
	a.users[1].artistsShown, a.users[1].podcastsShown = true, true
	return a
}

// Playlists are found by their name or their creator, in any case, and
// sorted with the liked songs kept first.
func TestFilterAndSortPlaylists(t *testing.T) {
	lists := []netease.Playlist{
		{ID: 1, Name: "我喜欢的音乐", SpecialType: 5, TrackCount: 3},
		{ID: 2, Name: "Night Drive", TrackCount: 40},
		{ID: 3, Name: "afternoon", TrackCount: 9},
		{ID: 4, Name: "杂", TrackCount: 12},
	}
	lists[3].Creator.Nickname = "Nightingale"
	if got := playlistIDs(filterPlaylists(lists, mineQuery("  NIGHT "))); !slices.Equal(got, []int64{2, 4}) {
		t.Errorf("searching night found %v", got)
	}
	if got := playlistIDs(filterPlaylists(lists, "")); len(got) != 4 {
		t.Errorf("no search kept %v", got)
	}
	if got := playlistIDs(sortPlaylists(lists, "name")); !slices.Equal(got, []int64{1, 3, 2, 4}) {
		t.Errorf("by name: %v", got)
	}
	if got := playlistIDs(sortPlaylists(lists, "songs")); !slices.Equal(got, []int64{1, 2, 4, 3}) {
		t.Errorf("by songs: %v", got)
	}
	if got := playlistIDs(sortPlaylists(lists, "")); !slices.Equal(got, []int64{1, 2, 3, 4}) {
		t.Errorf("NetEase's order: %v", got)
	}
}

// Searched, the albums of an artist found come first, then those found by
// their own name, each once.
func TestFilterAlbums(t *testing.T) {
	albums := []netease.AlbumProfile{
		{Album: netease.Album{ID: 1, Name: "Go Go"}, Artists: []netease.Artist{{Name: "Other"}}},
		{Album: netease.Album{ID: 2, Name: "Blue"}, Artists: []netease.Artist{{Name: "MyGO!!!!!"}}},
		{Album: netease.Album{ID: 3, Name: "Go"}, Artists: []netease.Artist{{Name: "Guest"}, {Name: "MyGO!!!!!"}}},
		{Album: netease.Album{ID: 4, Name: "Red"}, Artists: []netease.Artist{{Name: "Ave Mujica"}}},
	}
	var ids []int64
	for _, al := range filterAlbums(albums, mineQuery("go")) {
		ids = append(ids, al.ID)
	}
	if !slices.Equal(ids, []int64{2, 3, 1}) {
		t.Errorf("searching go found %v", ids)
	}
	if len(filterAlbums(albums, "")) != 4 {
		t.Error("no search left albums out")
	}
}

// Artists are found by a name they go by too, and podcasts by their host.
func TestFilterArtistsAndPodcasts(t *testing.T) {
	artists := []netease.ArtistProfile{
		{Artist: netease.Artist{ID: 1, Name: "長崎そよ"}, Alias: []string{"Soyo"}},
		{Artist: netease.Artist{ID: 2, Name: "Other"}},
	}
	if got := filterArtists(artists, mineQuery("soyo")); len(got) != 1 || got[0].ID != 1 {
		t.Errorf("searching soyo found %+v", got)
	}
	pods := []netease.Podcast{{ID: 1, Name: "夜谈", DJ: netease.User{Nickname: "主播"}}, {ID: 2, Name: "早安"}}
	if got := filterPodcasts(pods, mineQuery("主播")); len(got) != 1 || got[0].ID != 1 {
		t.Errorf("searching the host found %+v", got)
	}
}

// The sidebar lists the first few of a group, with a row to the listener's
// page, that kind opened there; set to, it lists them all.
func TestSidebarListsTheFirstFew(t *testing.T) {
	a := mineApp(t, 12)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if !tt.HasText(fmt.Sprintf("歌单 %d", sidebarShown)) || tt.HasText(fmt.Sprintf("歌单 %d", sidebarShown+1)) {
		t.Fatalf("the sidebar does not list just the first %d", sidebarShown)
	}
	if err := tt.Click("查看全部 12 个"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.route != (route{page: pageUser, id: 1}) || !a.mineView.open[mineMade] {
		t.Fatalf("查看全部 opened %+v, opened kinds %v", a.route, a.mineView.open)
	}
	if !tt.HasText("歌单 12") || !tt.HasText("收起") {
		t.Error("the page does not show the playlists opened")
	}

	b := mineApp(t, 12)
	b.settings.SidebarAll = true
	bt := ui.NewTester(b.view, 1280, 820)
	bt.Frame()
	if !bt.HasText("歌单 12") || bt.HasText("查看全部 12 个") {
		t.Error("set to list them all, the sidebar still cuts the list short")
	}
}

// On the listener's page, each kind shows a row until it is opened, and
// closes again.
func TestMinePageFolds(t *testing.T) {
	a := mineApp(t, 14)
	a.route = route{page: pageUser, id: 1}
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	// The sidebar lists the first 8; the page's first row is fewer.
	if !tt.HasText("展开全部 14") || tt.HasText("歌单 14") {
		t.Fatalf("closed, the page shows %v", tt.Texts())
	}
	if err := tt.Click("展开全部 14"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.mineView.open[mineMade] || !tt.HasText("歌单 14") {
		t.Fatal("opened, the page does not show the rest")
	}
	if err := tt.Click("收起"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.mineView.open[mineMade] || tt.HasText("歌单 14") {
		t.Error("closed again, the page still shows the rest")
	}
	if !tt.HasText("收藏的专辑") || !tt.HasText("关注的歌手") || !tt.HasText("订阅的播客") {
		t.Errorf("the page leaves out a kind: %v", tt.Texts())
	}
}

// A search on the page finds in every kind at once, leaving out those it
// finds nothing in, and opens what it finds.
func TestMinePageSearches(t *testing.T) {
	a := mineApp(t, 14)
	a.route = route{page: pageUser, id: 1}
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	a.mineView.query = "gam"
	tt.Frame()
	if tt.HasText("创建的歌单") || tt.HasText("订阅的播客") || !tt.HasText("收藏的专辑") || !tt.HasText("关注的歌手") {
		t.Errorf("searching gam shows %v", tt.Texts())
	}
	a.mineView.query = "歌单 1"
	tt.Frame()
	// 歌单 1 is the liked songs: 歌单 10 to 14 are found, past the first row.
	if !tt.HasText("歌单 14") || tt.HasText("展开全部 5") {
		t.Errorf("searching 歌单 1 shows %v", tt.Texts())
	}
	a.mineView.query = "nothing"
	tt.Frame()
	if !tt.HasText("没有找到包含「nothing」的歌单、专辑、歌手或播客") {
		t.Errorf("searching what is not there shows %v", tt.Texts())
	}
}

// With NM_FRAMES set, saves the listener's page, closed, opened and
// searched, in the light and the dark, for looking at it.
func TestMinePageFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	save := func(name string, tt *ui.Tester) {
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := mineApp(t, 14)
		a.route = route{page: pageUser, id: 1}
		tt := ui.NewTester(a.view, 1280, 820)
		tt.SetDark(dark)
		save("mine-"+appearance, tt)
		a.mineView.open = map[string]bool{mineMade: true}
		save("mine-open-"+appearance, tt)
		a.mineView.query = "a"
		save("mine-search-"+appearance, tt)
		a.mineView.only, a.mineView.query = onlyAlbums, "gam"
		save("mine-only-albums-"+appearance, tt)
	}
}

// Narrowed to a kind, the page shows only it, all of it; narrowed to the
// albums, an artist's name finds theirs.
func TestMinePageNarrows(t *testing.T) {
	a := mineApp(t, 14)
	a.route = route{page: pageUser, id: 1}
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if err := tt.Click("歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.mineView.only != onlyPlaylists || !tt.HasText("歌单 14") || tt.HasText("展开全部 14") || tt.HasText("收藏的专辑") {
		t.Fatalf("narrowed to playlists, the page shows %v", tt.Texts())
	}
	a.mineView.only, a.mineView.query = onlyAlbums, "gam"
	tt.Frame()
	if !tt.HasText("收藏的专辑") || tt.HasText("关注的歌手") || len(filterAlbums(a.sortedAlbums(), mineQuery(a.mineView.query))) != 1 {
		t.Errorf("narrowed to albums, gam shows %v", tt.Texts())
	}
	a.mineView.only, a.mineView.query = onlyArtists, "zebra"
	tt.Frame()
	if !tt.HasText("没有找到包含「zebra」的歌手") {
		t.Errorf("narrowed to artists, zebra shows %v", tt.Texts())
	}
}
