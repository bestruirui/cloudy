package player

import "testing"

// Cover is the cover of the song playing only: one fetched for the song
// before is not shown for the next.
func TestCoverOfTrackPlaying(t *testing.T) {
	p := queued(0, 1, 2)
	if p.Cover() != nil {
		t.Fatal("a cover before any was fetched")
	}
	p.cover, p.coverOf = []byte("jpeg"), 1
	if string(p.Cover()) != "jpeg" {
		t.Fatalf("cover %q", p.Cover())
	}
	next := p.queue[1]
	p.track = &next
	if p.Cover() != nil {
		t.Error("the song before's cover shown for the next")
	}
}
