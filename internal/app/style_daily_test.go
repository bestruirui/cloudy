package app

import (
	"slices"
	"testing"

	"netease-music-mygo/internal/netease"
)

// Tags are picked in one category: a second in it adds to the first, one
// of another starts again, and one picked again is unpicked.
func TestPickStyle(t *testing.T) {
	a := newTestApp()
	s := &a.style
	s.cat, s.picked = 1000, []int64{10013}
	a.pickStyle(1000, 10002)
	if s.cat != 1000 || !slices.Equal(s.picked, []int64{10013, 10002}) {
		t.Fatalf("same category: %d %v", s.cat, s.picked)
	}
	a.pickStyle(1000, 10013)
	if !slices.Equal(s.picked, []int64{10002}) {
		t.Fatalf("unpicked: %v", s.picked)
	}
	a.pickStyle(3000, 30011)
	if s.cat != 3000 || !slices.Equal(s.picked, []int64{30011}) {
		t.Fatalf("other category: %d %v", s.cat, s.picked)
	}
	if a.pickedIn(1000) != 0 || a.pickedIn(3000) != 1 {
		t.Fatalf("counts %d %d", a.pickedIn(1000), a.pickedIn(3000))
	}
}

// The dialog opens on the styles picked, and saving keeps them with the
// account before loading the songs in them.
func TestSaveStyles(t *testing.T) {
	a := newTestApp()
	picked := netease.StyleCategory{ID: 1000, Tags: []netease.StyleTag{{ID: 10013, Name: "R&B"}, {ID: 10002, Name: "欧美流行"}}}
	a.style.songs = netease.StyleSongs{Picked: picked}
	a.style.cats = []netease.StyleCategory{{ID: 1000}, {ID: 3000}}
	a.openStylePicker()
	if !a.style.open || a.style.tab != 1000 || !slices.Equal(a.style.picked, []int64{10013, 10002}) {
		t.Fatalf("opened on %d %v", a.style.tab, a.style.picked)
	}
	if got := styleNames(picked); got != "R&B · 欧美流行" {
		t.Fatalf("names %q", got)
	}

	var savedCat int64
	var savedTags []int64
	dailyStyleSongs = func() (netease.StyleSongs, error) { return netease.StyleSongs{}, nil }
	defer func() {
		saveStyleTags, dailyStyleSongs = netease.SaveStyleTags, netease.DailyStyleSongs
	}()
	a.pickStyle(3000, 30011)
	done := make(chan struct{})
	saveStyleTags = func(cat int64, tags []int64) error {
		savedCat, savedTags = cat, tags
		close(done)
		return nil
	}
	a.saveStyles()
	<-done
	if savedCat != 3000 || !slices.Equal(savedTags, []int64{30011}) {
		t.Fatalf("saved %d %v", savedCat, savedTags)
	}
}
