package player

import (
	"slices"
	"testing"

	"netease-music-mygo/internal/netease"
)

// queued returns a player with tracks of ids queued, the one at at playing,
// without loading it.
func queued(at int, ids ...int64) *Player {
	p := New(func(func()) {})
	for i, id := range ids {
		p.queue = append(p.queue, netease.Track{ID: id})
		p.order = append(p.order, i)
	}
	p.at = at
	t := p.queue[at]
	p.track = &t
	return p
}

// playOrder is the IDs of the queue in play order.
func playOrder(p *Player) []int64 {
	var out []int64
	for _, k := range p.order {
		out = append(out, p.queue[k].ID)
	}
	return out
}

// Songs asked to play next go right after the one playing, in their order;
// one already queued moves there rather than playing twice.
func TestPlayNext(t *testing.T) {
	p := queued(1, 1, 2, 3, 4)
	n := p.PlayNext([]netease.Track{{ID: 9}, {ID: 4}, {ID: 2}})
	if n != 2 {
		t.Errorf("placed %d, want 2: the song playing stays where it is", n)
	}
	if got := playOrder(p); !slices.Equal(got, []int64{1, 2, 9, 4, 3}) {
		t.Errorf("order %v", got)
	}
	if p.queue[p.order[p.at]].ID != 2 {
		t.Errorf("playing %d, want 2", p.queue[p.order[p.at]].ID)
	}
}

// Removing a song keeps the one playing; the song playing cannot go.
func TestRemoveAt(t *testing.T) {
	p := queued(2, 1, 2, 3, 4)
	if p.RemoveAt(2) {
		t.Error("removed the song playing")
	}
	if !p.RemoveAt(0) {
		t.Fatal("did not remove the first")
	}
	if got := playOrder(p); !slices.Equal(got, []int64{2, 3, 4}) {
		t.Errorf("order %v", got)
	}
	if p.queue[p.order[p.at]].ID != 3 {
		t.Errorf("playing %d, want 3", p.queue[p.order[p.at]].ID)
	}
}

// Dragged rows land before the row they are dropped on, and the song
// playing goes on being the one marked.
func TestReorder(t *testing.T) {
	p := queued(1, 1, 2, 3, 4, 5)
	p.Reorder([]int{3, 4}, 1)
	if got := playOrder(p); !slices.Equal(got, []int64{1, 4, 5, 2, 3}) {
		t.Errorf("order %v", got)
	}
	if p.queue[p.order[p.at]].ID != 2 {
		t.Errorf("playing %d, want 2", p.queue[p.order[p.at]].ID)
	}
	p.Reorder([]int{0}, 5)
	if got := playOrder(p); !slices.Equal(got, []int64{4, 5, 2, 3, 1}) {
		t.Errorf("to the end: order %v", got)
	}
}

// Clearing what is to come leaves the song playing alone.
func TestClearUpcoming(t *testing.T) {
	p := queued(2, 1, 2, 3, 4)
	p.ClearUpcoming()
	if got := playOrder(p); !slices.Equal(got, []int64{3}) || p.at != 0 {
		t.Errorf("order %v at %d", got, p.at)
	}
}
