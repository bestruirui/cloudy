package netease

import (
	"errors"
	"testing"
)

// pagesOf serves lists as NetEase does, playlistPage at a time.
func pagesOf(lists []Playlist, calls *[]int) func(int) ([]Playlist, bool, error) {
	return func(offset int) ([]Playlist, bool, error) {
		*calls = append(*calls, offset)
		end := min(offset+playlistPage, len(lists))
		if offset >= end {
			return nil, false, nil
		}
		return lists[offset:end], end < len(lists), nil
	}
}

func numbered(n int) []Playlist {
	lists := make([]Playlist, n)
	for i := range lists {
		lists[i].ID = int64(i + 1)
	}
	return lists
}

func TestAllPlaylistPagesLoadsPastTheFirstPage(t *testing.T) {
	var calls []int
	got, err := allPlaylistPages(pagesOf(numbered(450), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 450 {
		t.Fatalf("loaded %d playlists, want 450", len(got))
	}
	if len(calls) != 3 || calls[1] != playlistPage || calls[2] != 2*playlistPage {
		t.Fatalf("asked at %v", calls)
	}
}

func TestAllPlaylistPagesStopsWhenNoMore(t *testing.T) {
	var calls []int
	got, _ := allPlaylistPages(pagesOf(numbered(12), &calls))
	if len(got) != 12 || len(calls) != 1 {
		t.Fatalf("loaded %d in %d calls", len(got), len(calls))
	}
}

func TestAllPlaylistPagesKeepsAMovedPlaylistOnce(t *testing.T) {
	first := numbered(playlistPage)
	got, _ := allPlaylistPages(func(offset int) ([]Playlist, bool, error) {
		if offset == 0 {
			return first, true, nil
		}
		// One made meanwhile pushed the last of the first page into this one.
		return []Playlist{first[len(first)-1], {ID: 9999}}, false, nil
	})
	if len(got) != playlistPage+1 {
		t.Fatalf("loaded %d playlists, want %d", len(got), playlistPage+1)
	}
}

func TestAllPlaylistPagesErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := allPlaylistPages(func(int) ([]Playlist, bool, error) { return nil, false, boom }); err != boom {
		t.Fatalf("first page failing: err = %v", err)
	}
	got, err := allPlaylistPages(func(offset int) ([]Playlist, bool, error) {
		if offset == 0 {
			return numbered(playlistPage), true, nil
		}
		return nil, false, boom
	})
	if err != nil || len(got) != playlistPage {
		t.Fatalf("a later page failing: %d playlists, err = %v", len(got), err)
	}
}

func TestAllPlaylistPagesHasALimit(t *testing.T) {
	var calls int
	allPlaylistPages(func(offset int) ([]Playlist, bool, error) {
		calls++
		return []Playlist{{ID: int64(offset + 1)}}, true, nil
	})
	if calls != playlistPages {
		t.Fatalf("asked %d times, want %d", calls, playlistPages)
	}
}
