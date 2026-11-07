package app

import (
	"testing"

	"netease-music-mygo/internal/netease"
)

// A file downloaded before Cloudy wrote tags is found again on NetEase by
// its title, an artist and its length; a cover version or a live take of
// another length is not taken for it.
func TestMatchDownload(t *testing.T) {
	old := searchSongs
	defer func() { searchSongs = old }()
	searchSongs = func(q string, limit int) ([]netease.Track, error) {
		return []netease.Track{
			{ID: 1, Name: "Terranova", Artists: []netease.Artist{{Name: "Someone Else"}}, Duration: 233000},
			{ID: 2, Name: "Terranova (Live)", Artists: []netease.Artist{{Name: "Janina"}}, Duration: 233000},
			{ID: 3, Name: "terranova", Artists: []netease.Artist{{Name: "Janina"}}, Duration: 260000},
			{ID: 4, Name: "Terranova", Artists: []netease.Artist{{Name: "Janina"}}, Duration: 234500},
		}, nil
	}
	file := netease.Track{ID: -5, Name: "Terranova", Artists: []netease.Artist{{Name: "Janina"}}, Duration: 233000}
	got, ok := matchDownload(file)
	if !ok || got.ID != 4 {
		t.Fatalf("matched %v %v, want song 4", got.ID, ok)
	}
	file.Artists = []netease.Artist{{Name: "未知歌手"}}
	if _, ok := matchDownload(file); ok {
		t.Error("matched a file whose artist is unknown")
	}
}
