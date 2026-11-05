package player

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

func settleRetries(ch chan func()) {
	for {
		synctest.Wait()
		if len(ch) == 0 {
			return
		}
		for len(ch) > 0 {
			(<-ch)()
		}
	}
}

// A longer outage must keep retrying the same song with a capped delay, without pausing it.
func TestRetryOutageKeepsLoading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		calls := 0
		songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
			calls++
			return nil, context.DeadlineExceeded
		}
		p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}})
		p.Next()
		settleRetries(ch)
		for i, delay := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second} {
			time.Sleep(delay - time.Millisecond)
			settleRetries(ch)
			if calls != i+1 {
				t.Fatalf("retried too early: calls=%d, want %d", calls, i+1)
			}
			time.Sleep(time.Millisecond)
			settleRetries(ch)
			if calls != i+2 || p.Current().ID != 2 || !p.Loading() || !p.Playing() {
				t.Fatalf("outage stopped or skipped: calls=%d id=%d loading=%v playing=%v", calls, p.Current().ID, p.Loading(), p.Playing())
			}
		}
	})
}

// Pausing must cancel a retry, including one whose timer has already posted to the UI.
func TestRetryPauseCancels(t *testing.T) {
	for _, posted := range []bool{false, true} {
		t.Run(map[bool]string{false: "waiting", true: "posted"}[posted], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				quietNavigation(t)
				calls := 0
				songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
					calls++
					return nil, context.DeadlineExceeded
				}
				p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}})
				p.Next()
				settleRetries(ch)
				if posted {
					time.Sleep(500 * time.Millisecond)
					synctest.Wait()
				}
				p.TogglePlay()
				settleRetries(ch)
				time.Sleep(20 * time.Second)
				settleRetries(ch)
				if calls != 1 || p.Playing() || p.Loading() {
					t.Fatalf("retry ignored pause: calls=%d loading=%v playing=%v", calls, p.Loading(), p.Playing())
				}
				p.TogglePlay()
				settleRetries(ch)
				if calls != 2 || !p.Playing() || !p.Loading() {
					t.Fatal("Play did not resume loading the selected song")
				}
			})
		})
	}
}

// Next, Previous and a direct choice must supersede even an already queued retry.
func TestRetryNavigationCancels(t *testing.T) {
	for _, action := range []string{"next", "previous", "jump"} {
		t.Run(action, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				quietNavigation(t)
				var asked []int64
				songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
					asked = append(asked, id)
					if id == 2 {
						return nil, context.DeadlineExceeded
					}
					return nil, &netease.StreamError{Code: 301}
				}
				p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
				p.Jump(1)
				settleRetries(ch)
				time.Sleep(500 * time.Millisecond)
				synctest.Wait()
				want := int64(3)
				switch action {
				case "next":
					p.Next()
				case "previous":
					p.Prev()
					want = 1
				case "jump":
					p.Jump(3)
					want = 4
				}
				settleRetries(ch)
				time.Sleep(20 * time.Second)
				settleRetries(ch)
				if len(asked) < 2 || p.Current().ID != want {
					t.Fatalf("old retry replaced %s: asked=%v current=%d", action, asked, p.Current().ID)
				}
				for _, id := range asked[1:] {
					if id != want {
						t.Fatalf("old retry asked for %d after choosing %d", id, want)
					}
				}
			})
		})
	}
}

// A reconnect after interrupted playback must resume its position and honor a seek made while loading.
func TestRetryInterruptedAudioResumes(t *testing.T) {
	for _, seek := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume", true: "seek"}[seek], func(t *testing.T) {
			quietNavigation(t)
			st := retryAudio(t)
			calls := 0
			songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
				calls++
				return st, nil
			}
			p, ch := navigationPlayer(t, []netease.Track{{ID: 1, Duration: 5000}, {ID: 2}})
			dl := downloaded(nil)
			dl.err = io.ErrUnexpectedEOF
			s := &checkedStream{stream: &mp3Stream{dl: dl, drop: dl.Close}}
			p.stream, p.out, p.playing = s, audio.NewPlayer(silence{}), true
			want := 2 * time.Second
			p.out.Seek(want)
			p.finish(s)
			if seek {
				want = 3 * time.Second
				p.Seek(want)
			}
			finishNavigation(t, p, ch)
			if calls != 1 || !p.Ready() || !p.Playing() || p.Current().ID != 1 || p.Position() < want {
				t.Fatalf("reconnect lost playback: calls=%d ready=%v playing=%v id=%d position=%v", calls, p.Ready(), p.Playing(), p.Current().ID, p.Position())
			}
		})
	}
}

// Only explicit unavailability or cancellation stops an address retry, regardless of its code.
func TestRetryErrorPolicy(t *testing.T) {
	for _, c := range []struct {
		name    string
		err     error
		attempt int
		want    bool
	}{
		{"cancelled", context.Canceled, 0, false},
		{"unavailable", netease.ErrUnavailable, 0, false},
		{"login_like", &netease.StreamError{Code: 301}, 20, true},
		{"unknown", &netease.StreamError{Code: 400}, 20, true},
		{"missing_code", &netease.StreamError{Code: 0}, 20, true},
		{"empty_address", &netease.StreamError{Code: 200}, 20, true},
		{"negative_code", &netease.StreamError{Code: -460}, 20, true},
		{"parse_failure", &netease.StreamError{Cause: errors.New("invalid response")}, 20, true},
		{"service", &netease.StreamError{Code: 503}, 20, true},
		{"limited", &netease.StreamError{Code: 429}, 20, true},
		{"timeout", context.DeadlineExceeded, 20, true},
		{"truncated", io.ErrUnexpectedEOF, 20, true},
		{"cdn", &streamHTTPError{code: 503}, 20, true},
		{"missing", &streamHTTPError{code: 404}, 0, false},
		{"expired_url", &streamHTTPError{code: 403}, 0, true},
		{"persistent_decode", errors.New("bad frame"), 3, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if delay, ok := retryWait(c.err, c.attempt); ok != c.want || (ok && delay > 8*time.Second) {
				t.Fatalf("retry=%v delay=%v, want retry=%v", ok, delay, c.want)
			}
		})
	}
}

func retryAudio(t *testing.T) *netease.Stream {
	t.Helper()
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport
	http.DefaultTransport = navigationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "audio.test" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(b)), Request: r}, nil
		}
		return transport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	return &netease.Stream{URL: "https://audio.test/song", Size: int64(len(b)), Type: "mp3", Level: "exhigh"}
}

// A brief failed request must reload the same song and play without another click.
func TestRetryNetworkRecoversWithoutInput(t *testing.T) {
	quietNavigation(t)
	st := retryAudio(t)
	calls := 0
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		calls++
		if id != 2 {
			t.Errorf("retry asked for song %d", id)
		}
		if calls == 1 {
			return nil, context.DeadlineExceeded
		}
		return st, nil
	}
	p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2, Duration: 5000}, {ID: 3}})
	p.Next()
	finishNavigation(t, p, ch)
	if calls != 2 || !p.Ready() || p.Loading() || !p.Playing() || p.Current().ID != 2 {
		t.Fatalf("did not recover automatically: calls=%d ready=%v loading=%v playing=%v", calls, p.Ready(), p.Loading(), p.Playing())
	}
}

type retryBody struct {
	ctx    context.Context
	chunks chan []byte
}

func (b *retryBody) Read(dst []byte) (int, error) {
	select {
	case chunk := <-b.chunks:
		return copy(dst, chunk), nil
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	}
}

func (*retryBody) Close() error { return nil }

// Download activity extends its deadline; twenty seconds without bytes must unblock a reconnect.
func TestRetryStalledDownloadTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		quietNavigation(t)
		chunks := make(chan []byte, 2)
		chunks <- []byte("first")
		http.DefaultTransport = navigationTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &retryBody{ctx: r.Context(), chunks: chunks}, Request: r}, nil
		})
		dl := startDownload("https://audio.test/stall", 0, nil, nil)
		defer dl.Close()
		dl.waitFor(5)
		time.Sleep(15 * time.Second)
		chunks <- []byte("second")
		dl.waitFor(11)
		time.Sleep(19 * time.Second)
		synctest.Wait()
		dl.mu.Lock()
		done := dl.done
		dl.mu.Unlock()
		if done {
			t.Fatal("active download timed out")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		dl.mu.Lock()
		err, done := dl.err, dl.done
		dl.mu.Unlock()
		if !done || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled download did not time out: done=%v error=%v", done, err)
		}
	})
}

// A failed CDN response must preserve its status so the same song can reconnect automatically.
func TestRetryDownloadServiceErrorRecovers(t *testing.T) {
	quietNavigation(t)
	st := retryAudio(t)
	transport := http.DefaultTransport
	requests := 0
	http.DefaultTransport = navigationTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "audio.test" {
			requests++
			if requests == 1 {
				return &http.Response{StatusCode: 503, Status: "503 Service Unavailable", Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
			}
		}
		return transport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) { return st, nil }
	p, ch := navigationPlayer(t, []netease.Track{{ID: 1}, {ID: 2, Duration: 5000}, {ID: 3}})
	p.Next()
	finishNavigation(t, p, ch)
	if requests != 2 || !p.Ready() || !p.Playing() || p.Current().ID != 2 {
		t.Fatalf("CDN failure did not reconnect: requests=%d ready=%v playing=%v id=%d", requests, p.Ready(), p.Playing(), p.Current().ID)
	}
}
