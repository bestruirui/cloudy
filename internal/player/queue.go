package player

import (
	"slices"

	"netease-music-mygo/internal/netease"
)

// The queue as the listener edits it: in play order, as Now Playing lists
// it. Positions are of the play order, not of the queue as it was given.

// PlayNext puts tracks right after the one playing, in their order: those
// already queued move there, the others join the queue. With nothing
// playing, the first of them starts. It returns how many it placed.
func (p *Player) PlayNext(tracks []netease.Track) int {
	if len(tracks) == 0 {
		return 0
	}
	cur := int64(0)
	if p.track != nil {
		cur = p.track.ID
	}
	placed := 0
	var picked []int // queue indices, in the order asked
	seen := map[int64]bool{}
	for _, t := range tracks {
		if t.ID == cur || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		k := slices.IndexFunc(p.queue, func(q netease.Track) bool { return q.ID == t.ID })
		if k < 0 {
			p.queue = append(p.queue, t)
			k = len(p.queue) - 1
		} else if pos := slices.Index(p.order, k); pos >= 0 {
			p.order = slices.Delete(p.order, pos, pos+1)
			if pos < p.at {
				p.at--
			}
		}
		picked = append(picked, k)
		placed++
	}
	if placed == 0 {
		return 0
	}
	p.order = slices.Insert(p.order, p.at+1, picked...)
	if p.track == nil {
		p.load(max(p.at, 0))
		return placed
	}
	p.sync()
	return placed
}

// RemoveAt takes the track at pos of the play order out of the queue. The
// track playing stays: there is nothing to play in its place.
func (p *Player) RemoveAt(pos int) bool {
	if pos < 0 || pos >= len(p.order) || pos == p.at {
		return false
	}
	k := p.order[pos]
	p.order = slices.Delete(p.order, pos, pos+1)
	p.queue = slices.Delete(p.queue, k, k+1)
	for i, v := range p.order {
		if v > k {
			p.order[i] = v - 1
		}
	}
	if pos < p.at {
		p.at--
	}
	p.refill()
	p.sync()
	return true
}

// Reorder moves the tracks at rows of the play order, in their order, to
// before the track at to, or to the end for len(order), as a list dragged
// says: the track playing goes on playing where it lands.
func (p *Player) Reorder(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	moving := map[int]bool{}
	var moved []int
	for _, r := range rows {
		if r >= 0 && r < len(p.order) && !moving[r] {
			moving[r] = true
			moved = append(moved, p.order[r])
		}
	}
	cur := -1
	if p.at >= 0 && p.at < len(p.order) {
		cur = p.order[p.at]
	}
	kept := make([]int, 0, len(p.order))
	at := 0
	for i, v := range p.order {
		if i == to {
			at = len(kept)
		}
		if !moving[i] {
			kept = append(kept, v)
		}
	}
	if to >= len(p.order) {
		at = len(kept)
	}
	p.order = slices.Insert(kept, at, moved...)
	if cur >= 0 {
		p.at = slices.Index(p.order, cur)
	}
	p.refill()
	p.sync()
}

// ClearUpcoming leaves only the track playing in the queue, and ends a
// radio, which would only fill it again.
func (p *Player) ClearUpcoming() {
	if p.radio != "" || p.feed != nil {
		p.endRadio()
	}
	if p.track == nil || p.at < 0 || p.at >= len(p.order) {
		p.queue, p.order, p.at = nil, nil, -1
		p.sync()
		return
	}
	p.queue = []netease.Track{p.queue[p.order[p.at]]}
	p.order, p.at = []int{0}, 0
	p.sync()
}
