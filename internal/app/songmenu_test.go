package app

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func TestSongMenuCopies(t *testing.T) {
	var copied string
	real := copyText
	copyText = func(s string) error { copied = s; return nil }
	defer func() { copyText = real }()

	a := newTestApp()
	a.settings = defaultSettings()
	tr := netease.Track{ID: 1859245776, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}, {Name: "小明"}}, Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()

	for _, c := range []struct{ item, want string }{
		{"复制歌名", "宝船"},
		{"复制歌名和歌手", "宝船 - DOUDOU, 小明"},
		{"复制链接", "https://music.163.com/song?id=1859245776"},
	} {
		if err := tt.Click("更多"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if err := tt.Click(c.item); err != nil {
			t.Fatalf("%v; the view shows %q", err, tt.Texts())
		}
		tt.Frame()
		if copied != c.want {
			t.Errorf("%s copied %q, want %q", c.item, copied, c.want)
		}
		if a.menu.open {
			t.Errorf("%s left the menu open", c.item)
		}
	}
}

// 添加到歌单 lists the playlists in the pane of the menu, in place of what it
// showed, and the row back shows that again.
func TestSongMenuPlaylists(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.profile = &netease.Profile{UserID: 1}
	a.mine = []netease.Playlist{{ID: 7, Name: "钢琴曲"}}
	tr := netease.Track{ID: 1, Name: "宝船", Duration: 200000}
	a.pl.Cue([]netease.Track{tr})
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("更多"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("添加到歌单"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("钢琴曲") || tt.HasText("复制歌名") {
		t.Errorf("the playlists are not all it shows: %q", tt.Texts())
	}
	if err := tt.Click("返回"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("复制歌名") || tt.HasText("钢琴曲") {
		t.Errorf("back did not show the menu: %q", tt.Texts())
	}
}

func TestSongMenuOnRightClick(t *testing.T) {
	var copied string
	real := copyText
	copyText = func(s string) error { copied = s; return nil }
	defer func() { copyText = real }()

	a := newTestApp()
	a.settings = defaultSettings()
	a.playlists[42] = &netease.Playlist{ID: 42, Name: "飙升榜", Tracks: []netease.Track{
		{ID: 1, Name: "一张相片", Artists: []netease.Artist{{Name: "林宥嘉"}}, Duration: 200000},
		{ID: 2, Name: "宝船", Artists: []netease.Artist{{Name: "DOUDOU"}}, Duration: 200000},
	}}
	a.route = route{page: pagePlaylist, id: 42}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if err := tt.RightClick("宝船"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, label := range []string{"复制歌名", "复制歌名和歌手", "复制链接"} {
		if !slices.Contains(tt.Texts(), label) {
			t.Fatalf("the menu has no %s: %q", label, tt.Texts())
		}
	}
	if err := chooseItem(tt, "复制链接"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if want := "https://music.163.com/song?id=2"; copied != want {
		t.Errorf("copied %q, want %q", copied, want)
	}
	if a.pl.Current() != nil {
		t.Error("the right click played the song")
	}
}
