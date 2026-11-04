package app

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Choosing a category requests its official source, opens its cards, and reuses its cache.
func TestPlaylistFilterNavigation(t *testing.T) {
	oldCategory, oldQuality := categoryPlaylists, qualityPlaylists
	defer func() { categoryPlaylists, qualityPlaylists = oldCategory, oldQuality }()
	called := make(chan string, 8)
	categoryPlaylists = func(cat string, limit int) ([]netease.Playlist, error) { called <- cat; return nil, nil }
	qualityPlaylists = func(limit int) ([]netease.Playlist, error) { called <- "精品"; return nil, nil }
	a := newTestApp()
	a.settings = defaultSettings()
	for i := range 12 {
		a.featured = append(a.featured, netease.Playlist{ID: int64(i + 1), Name: fmt.Sprintf("推荐 %d", i)})
	}
	a.charts = []netease.Playlist{{ID: 99, Name: "榜单"}}
	tt := ui.NewTester(func(c *ui.Context) {
		a.pal = paletteFor(c.Theme())
		a.recommendedPlaylists(c)
	}, 1180, 700)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	for i, want := range []string{"精品", "华语", "R&B/Soul", "轻音乐", "电子", "学习", "运动"} {
		filter := i + 1
		if err := tt.Click("歌单分类：" + playlistFilters[filter].label); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-called:
			if got != want {
				t.Fatalf("category %d requested %s, want %s", filter, got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the category did not request playlists")
		}
		list := netease.Playlist{ID: int64(100 + filter), Name: fmt.Sprintf("分类歌单 %d", filter)}
		a.playlistFilterAnswered(a.accountGen, filter, a.filteredPlaylists[filter], []netease.Playlist{list}, nil)
		tt.Frame()
		if a.playlistFilter != filter {
			t.Fatal("the category selection changed while its response arrived")
		}
		a.playlists[list.ID] = &list
		if err := tt.Click(list.Name); err != nil {
			t.Fatal(err)
		}
		if a.route.page != pagePlaylist || a.route.id != list.ID {
			t.Fatal("the category's card opened another playlist")
		}
	}
	if err := tt.Click("歌单分类：推荐"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.playlistFilter != 0 || a.featured[0].ID != 1 || a.charts[0].ID != 99 {
		t.Fatal("category browsing replaced home recommendations or charts")
	}
	if err := tt.Click("歌单分类：华语"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-called:
		t.Fatalf("a cached category was fetched again: %s", got)
	default:
	}
}

// Late category responses stay in their own cache and cannot cross a reload or login.
func TestPlaylistFilterReplyOwnership(t *testing.T) {
	a := newTestApp()
	old := &playlistBrowseData{loading: true}
	other := &playlistBrowseData{loading: true}
	a.filteredPlaylists[2], a.filteredPlaylists[3] = old, other
	a.playlistFilter = 3
	gen := a.accountGen
	a.playlistFilterAnswered(gen, 2, old, []netease.Playlist{{ID: 42}}, nil)
	if a.playlistFilter != 3 || !other.loading || old.loading || old.lists[0].ID != 42 {
		t.Fatal("a late response changed the selected category")
	}
	fresh := &playlistBrowseData{loading: true}
	a.filteredPlaylists[2] = fresh
	a.playlistFilterAnswered(gen, 2, old, nil, errors.New("old request"))
	if !fresh.loading || fresh.err != "" {
		t.Fatal("a superseded request changed its replacement")
	}
	a.accountGen++
	a.playlistFilterAnswered(gen, 2, fresh, []netease.Playlist{{ID: 43}}, nil)
	if !fresh.loading || len(fresh.lists) != 0 {
		t.Fatal("a previous login supplied the current category")
	}
}

// A failed category retries in place; an empty success does not request on every frame.
func TestPlaylistFilterRetry(t *testing.T) {
	old := categoryPlaylists
	defer func() { categoryPlaylists = old }()
	called := make(chan string, 2)
	categoryPlaylists = func(cat string, limit int) ([]netease.Playlist, error) { called <- cat; return nil, nil }
	a := newTestApp()
	a.playlistFilter = 7
	a.filteredPlaylists[7] = &playlistBrowseData{err: "failed"}
	tt := ui.NewTester(func(c *ui.Context) { a.recommendedPlaylists(c) }, 1180, 700)
	tt.Frame()
	if err := tt.Click("重新加载"); err != nil {
		t.Fatal(err)
	}
	select {
	case cat := <-called:
		if cat != "运动" {
			t.Fatalf("retry requested %s", cat)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retry did not request playlists")
	}
	a.playlistFilterAnswered(a.accountGen, 7, a.filteredPlaylists[7], nil, nil)
	tt.Frame()
	tt.Frame()
	select {
	case <-called:
		t.Fatal("an empty category made a request while drawing")
	default:
	}
}

// With NM_FRAMES set, saves the category pills and cards in both appearances and widths.
func TestPlaylistFilterFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		for _, width := range []int{1180, 880} {
			a := newTestApp()
			a.settings = defaultSettings()
			a.playlistFilter = 3
			var lists []netease.Playlist
			for i := range 12 {
				pl := netease.Playlist{ID: int64(i + 1), Name: fmt.Sprintf("R&B / Soul · 夜晚的旋律 %d", i+1), CoverURL: fmt.Sprintf("https://example.com/browse-%d", i), PlayCount: 1234567}
				img := image.NewRGBA(image.Rect(0, 0, 64, 64))
				for y := range 64 {
					for x := range 64 {
						img.SetRGBA(x, y, color.RGBA{uint8(55 + i*12), uint8(80 + y), uint8(110 + x), 255})
					}
				}
				a.art.add(netease.Sized(pl.CoverURL, 400), &artwork{bmp: ui.NewBitmap(img), used: time.Now(), size: 1})
				lists = append(lists, pl)
			}
			a.filteredPlaylists[3] = &playlistBrowseData{lists: lists}
			tt := ui.NewTester(func(c *ui.Context) {
				a.pal = paletteFor(c.Theme())
				c.Root().Background(a.pal.bg)
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(sidebarWidth).Right(0).Padding(0, pagePad).Children(func() { a.recommendedPlaylists(c) })
			}, width, 650)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			name := fmt.Sprintf("playlist-filters-%d-%t.png", width, dark)
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
