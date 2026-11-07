package app

import (
	"testing"
	"time"
)

// A tap on the cover closes Now Playing, as the chevron does.
func TestTapCoverCloses(t *testing.T) {
	a, tt, r := coverApp(t)
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(60 * time.Millisecond)
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if a.nowPlaying {
		t.Fatal("still open after a tap on the cover")
	}
	if id := a.pl.Current().ID; id != 2 {
		t.Fatalf("a tap changed the song: playing %d", id)
	}
}

// A drag too short to fling springs back and is no tap: the view stays.
func TestDragCoverKeepsOpen(t *testing.T) {
	a, tt, r := coverApp(t)
	drag(tt, r, -r.W*0.3, 300*time.Millisecond)
	if !a.nowPlaying {
		t.Fatal("a drag closed the view")
	}
	if id := a.pl.Current().ID; id != 2 {
		t.Fatalf("a short drag changed the song: playing %d", id)
	}
}
