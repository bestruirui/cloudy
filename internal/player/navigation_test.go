package player

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

// A failed request must keep the chosen entry loading instead of skipping, in either direction.
func TestNavigationLoadFailureStaysSelected(t *testing.T) {
	for _, action := range []string{"next", "previous", "jump"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				quietNavigation(t)
				songURL = func(context.Context, int64, string) (*netease.Stream, error) { return nil, context.DeadlineExceeded }
				p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
				p.at, p.track = 1, &p.queue[1]
				want := int64(3)
				switch action {
				case "next":
					p.Next()
				case "previous":
					want = 1
					p.Prev()
				case "jump":
					want = 4
					p.Jump(3)
				}
				settleRetries(ch)
				if p.Current().ID != want || !p.Loading() || !p.Playing() {
					t.Fatalf("failed selection: id=%d want=%d loading=%v playing=%v", p.Current().ID, want, p.Loading(), p.Playing())
				}
			})
		})
	}
}

type navigationTransport func(*http.Request) (*http.Response, error)

func (f navigationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func quietNavigation(t *testing.T) {
	t.Helper()
	transport := http.DefaultTransport
	http.DefaultTransport = navigationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "music.163.com" || r.URL.Host == "interfacepc.music.163.com" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":200}`)), Request: r}, nil
		}
		return transport.RoundTrip(r)
	})
	old := songURL
	t.Cleanup(func() { songURL, http.DefaultTransport = old, transport })
}

func navigationPlayer(t *testing.T, tracks []netease.Track) (*Player, chan func()) {
	t.Helper()
	ch := make(chan func(), 1024)
	p := &Player{post: func(fn func()) { ch <- fn }, at: -1, volume: 0, gain: 1}
	p.Cue(tracks)
	t.Cleanup(p.stop)
	return p, ch
}

func cacheNavigationTrack(t *testing.T, p *Player, id int64) {
	t.Helper()
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if p.Cache == nil {
		p.Cache = NewCache(t.TempDir(), 1<<30)
		t.Cleanup(func() {
			out, s := p.out, p.stream
			p.stop()
			if s != nil {
				s.Close()
			}
			if out != nil {
				out.Close()
			}
		})
	}
	st := &netease.Stream{Size: int64(len(b)), Type: "mp3", Level: "exhigh"}
	f := p.Cache.part(st)
	if f == nil {
		t.Fatal("could not create cache fixture")
	}
	if _, err := f.Write(b); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, ok := p.Cache.keep(id, st, f.Name(), int64(len(b))); !ok {
		t.Fatal("could not cache fixture")
	}
}

func finishNavigation(t *testing.T, p *Player, ch chan func()) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for p.Loading() {
		select {
		case fn := <-ch:
			fn()
		case <-deadline.C:
			t.Fatal("load did not finish")
		}
	}
}

// An address rejection must retry the same selection automatically, even for a login-like code.
func TestNavigationAddressFailureKeepsLoading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		var asked []int64
		songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
			asked = append(asked, id)
			return nil, &netease.StreamError{Code: 301}
		}
		p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
		p.Next()
		settleRetries(ch)
		if p.Current().ID != 2 || !p.Loading() || !p.Playing() {
			t.Fatalf("address rejection stopped or moved: %+v", p.Current())
		}
		time.Sleep(500 * time.Millisecond)
		settleRetries(ch)
		if !reflect.DeepEqual(asked, []int64{2, 2}) || p.Current().ID != 2 {
			t.Fatalf("retry asked for %v, selected %d", asked, p.Current().ID)
		}
	})
}

// Unavailable songs used to reverse Previous into a forward jump, including in shuffle.
func TestNavigationUnavailableKeepsDirection(t *testing.T) {
	for _, direction := range []string{"next", "previous"} {
		t.Run(direction, func(t *testing.T) {
			quietNavigation(t)
			var asked []int64
			songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
				asked = append(asked, id)
				return nil, netease.ErrUnavailable
			}
			p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
			p.order = []int{0, 2, 1, 3}
			if direction == "next" {
				p.Next()
			} else {
				p.Prev()
			}
			finishNavigation(t, p, ch)
			want := []int64{3, 2, 4, 1}
			if direction == "previous" {
				want = []int64{4, 2, 3, 1}
			}
			if !reflect.DeepEqual(asked, want) || p.Playing() || p.Err() == "" {
				t.Fatalf("asked %v, want %v; playing=%v error=%q", asked, want, p.Playing(), p.Err())
			}
		})
	}
}

// A cache file with the right length but broken audio must be fetched again for the same song.
func TestNavigationCorruptCacheReloadsSameTrack(t *testing.T) {
	quietNavigation(t)
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	c := NewCache(t.TempDir(), 1<<30)
	st := &netease.Stream{Size: int64(len(b)), Type: "mp3", Level: "exhigh", URL: srv.URL}
	f := c.part(st)
	if _, err := f.Write(make([]byte, len(b))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, ok := c.keep(7, st, f.Name(), int64(len(b))); !ok {
		t.Fatal("could not seed cache")
	}
	var asked []int64
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		asked = append(asked, id)
		return st, nil
	}
	p := &Player{Cache: c}
	_, s, err := p.open(netease.Track{ID: 7, Duration: 5000}, "exhigh")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Read(make([]float32, 4096)); n == 0 || err != nil {
		t.Fatalf("replacement did not decode: n=%d error=%v", n, err)
	}
	if !reflect.DeepEqual(asked, []int64{7}) {
		t.Fatalf("replacement requested %v", asked)
	}
}

// Changing songs must cancel an old download stuck before its first audio bytes arrive.
func TestNavigationCancelsStalledLoad(t *testing.T) {
	quietNavigation(t)
	entered, cancelled := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer srv.Close()
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		return &netease.Stream{URL: srv.URL, Type: "mp3", Level: "exhigh"}, nil
	}
	p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2, Duration: 5000}})
	cacheNavigationTrack(t, p, 2)
	p.Jump(0)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	p.Next()
	finishNavigation(t, p, ch)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("old download was not cancelled")
	}
	for len(ch) > 0 {
		(<-ch)()
	}
	if p.Current().ID != 2 || !p.Playing() || p.Err() != "" {
		t.Fatalf("stale load changed the selection: id=%d error=%q", p.Current().ID, p.Err())
	}
}

// A delayed older result must not replace the last choice after Next, Next, Previous.
func TestNavigationLastSelectionWins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var calls atomic.Int32
		songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil, &netease.StreamError{Code: 301}
		}
		p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
		p.Next()
		<-entered
		p.Next()
		p.Prev()
		synctest.Wait()
		settleRetries(ch)
		close(release)
		synctest.Wait()
		for len(ch) > 0 {
			(<-ch)()
		}
		if p.Current().ID != 2 || !p.Playing() || !p.Loading() {
			t.Fatalf("last choice lost: id=%d loading=%v", p.Current().ID, p.Loading())
		}
	})
}

// A decoder or download failure must reconnect the selection instead of advancing the queue.
func TestNavigationBrokenStreamDoesNotAdvance(t *testing.T) {
	for _, failure := range []string{"decode", "download", "truncated"} {
		t.Run(failure, func(t *testing.T) {
			p, _ := navigationPlayer(t, []netease.Track{{ID: 1, Duration: 600000}, {ID: 2}})
			dl := downloaded(nil)
			s := &checkedStream{stream: &mp3Stream{dl: dl, drop: dl.Close, rate: 48000, frames: 48000 * 600}}
			switch failure {
			case "decode":
				s.err = errors.New("invalid audio frame")
			case "download":
				dl.err = io.ErrUnexpectedEOF
			}
			p.stream, p.out, p.playing = s, audio.NewPlayer(silence{}), true
			p.finish(s)
			if p.Current().ID != 1 || !p.Playing() || !p.Loading() || p.out != nil {
				t.Fatalf("broken stream advanced or stayed active: id=%d error=%q", p.Current().ID, p.Err())
			}
		})
	}
}

// A normally completed song must still advance once to the next queue entry.
func TestNavigationNaturalEndAdvances(t *testing.T) {
	quietNavigation(t)
	p, ch := navigationPlayer(t, []netease.Track{{ID: 1, Duration: 600000}, {ID: 2, Duration: 5000}})
	cacheNavigationTrack(t, p, 2)
	dl := downloaded(nil)
	s := &checkedStream{stream: &mp3Stream{dl: dl, drop: dl.Close}}
	p.stream, p.out, p.playing = s, audio.NewPlayer(silence{}), true
	p.out.Seek(599 * time.Second)
	p.finish(s)
	finishNavigation(t, p, ch)
	if p.Current().ID != 2 || !p.Playing() || p.Err() != "" {
		t.Fatalf("natural end did not play the next song: id=%d error=%q", p.Current().ID, p.Err())
	}
}

// Next at a radio's end must invalidate the old load while waiting for its feed.
func TestNavigationRadioWaitDropsOldLoad(t *testing.T) {
	quietNavigation(t)
	p, ch := navigationPlayer(t, []netease.Track{{ID: 1, Duration: 5000}})
	cacheNavigationTrack(t, p, 1)
	cacheNavigationTrack(t, p, 2)
	p.Jump(0)
	var got func([]netease.Track)
	p.AttachRadio("fm:DEFAULT", func(fn func([]netease.Track)) { got = fn })
	p.Next()
	next(t, ch)
	if p.Ready() || !p.awaiting || !p.Loading() {
		t.Fatal("old load resumed instead of waiting for the next radio song")
	}
	got([]netease.Track{{ID: 2, Duration: 5000}})
	finishNavigation(t, p, ch)
	if p.Current().ID != 2 || !p.Playing() || p.Err() != "" {
		t.Fatalf("radio did not continue: id=%d error=%q", p.Current().ID, p.Err())
	}
}

// A stalled initial attempt must time out into a reconnect, rather than stop or skip.
func TestNavigationLoadDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
		p.Next()
		time.Sleep(20 * time.Second)
		synctest.Wait()
		settleRetries(ch)
		if p.Current().ID != 2 || !p.Playing() || !p.Loading() || p.retryTimer == nil {
			t.Fatalf("deadline moved or hung the selection: id=%d error=%q", p.Current().ID, p.Err())
		}
	})
}
