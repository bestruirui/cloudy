package app

import (
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
	"netease-music-mygo/internal/player"
)

func newTestApp() *app {
	a := &app{
		playlists: map[int64]*netease.Playlist{}, loadingPl: map[int64]bool{}, plErr: map[int64]string{},
		artists: map[int64]*artistData{}, albums: map[int64]*albumData{}, users: map[int64]*userData{},
		podcasts: map[int64]*podcastData{}, podcastTab: &podcastTab{}, searches: map[route]*searchData{},
		lyricAt: -1, queueAt: -1, splashed: true,
	}
	a.art = newArtCache(func(func()) {})
	a.pl = player.New(func(func()) {})
	a.pl.SetVolume(0.5)
	return a
}

// chooseItem clicks through the rows of the menu open, each after the last
// has shown what it opens.
func chooseItem(tt *ui.Tester, path ...string) error {
	for _, label := range path {
		tt.Frame()
		if err := tt.Click(label); err != nil {
			return err
		}
	}
	tt.Frame()
	return nil
}
