package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/netease"
)

// An unlike failure after logout must neither panic nor put hearts in a new login.
func TestFixLikeReplyAfterLogout(t *testing.T) {
	a := newTestApp()
	a.profile = &netease.Profile{UserID: 1}
	a.liked = map[int64]bool{7: true}
	gen := a.accountGen
	a.setLiked(7, false)
	// The state reset by signedIn, without starting account requests.
	a.accountGen++
	a.profile, a.liked, a.likedPrograms = nil, nil, nil
	err := errors.New("offline")
	a.likeAnswered(gen, 7, false, err)
	if a.liked != nil || a.toast != "" {
		t.Fatal("a signed-out reply changed hearts or toast")
	}
	a.profile = &netease.Profile{UserID: 1} // the same account can sign in again
	a.likeAnswered(gen, 7, false, err)
	a.programLikeAnswered(gen, 9, false, err)
	if a.liked != nil || a.likedPrograms != nil || a.toast != "" {
		t.Fatal("an earlier login's reply changed the current one")
	}
	a.likeAnswered(a.accountGen, 7, false, err)
	if !a.liked[7] {
		t.Fatal("a current login's failed unlike was not rolled back")
	}
}

// An old playlist reply must not clear a new load, cache its tracks or start it.
func TestFixPlaylistReplyAfterAccountChange(t *testing.T) {
	a := newTestApp()
	gen := a.accountGen
	a.accountGen++
	a.loadingPl[42] = true
	old := &netease.Playlist{ID: 42, Tracks: []netease.Track{{ID: 1}}}
	a.playlistAnswered(gen, 42, old, nil, true)
	a.playlistAnswered(gen, 42, nil, errors.New("offline"), false)
	if !a.loadingPl[42] || a.playlists[42] != nil || a.plErr[42] != "" || a.pl.Current() != nil {
		t.Fatal("a stale playlist changed the new login or playback")
	}
	current := &netease.Playlist{ID: 42, Name: "current"}
	a.playlistAnswered(a.accountGen, 42, current, nil, false)
	if a.loadingPl[42] || a.playlists[42] != current {
		t.Fatal("the current playlist reply was not accepted")
	}
}

// A late station response cannot replace another queue, a new request or a login.
func TestFixRadioRequestOwnership(t *testing.T) {
	a := newTestApp()
	old := a.requestRadio(styleRadio("摇滚"))
	a.pl.AttachRadio("manual selection", func(func([]netease.Track)) {})
	if a.acceptRadio(old) || a.radioStarting != "" {
		t.Fatal("accepted a reply after playback was replaced")
	}
	old = a.requestRadio(styleRadio("摇滚"))
	latest := a.requestRadio(styleRadio("摇滚"))
	if a.acceptRadio(old) || a.radioStarting == "" || !a.acceptRadio(latest) {
		t.Fatal("same-name requests were not distinguished")
	}
	old = a.requestRadio(heartRadio)
	a.accountGen++
	if a.acceptRadio(old) {
		t.Fatal("accepted the previous account's heart-mode response")
	}
	old = a.requestRadio(styleRadio("伤感"))
	a.exitRadio()
	if a.acceptRadio(old) {
		t.Fatal("accepted a response after cancelling the station")
	}
}

// Style tiles toggle independently from the language, including while loading.
func TestRoamStyleLanguageToggle(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.roam = &roamer{pools: map[string][]netease.Playlist{}, dealing: map[string][]netease.Track{}, played: map[int64]bool{}}
	for i, label := range []string{"伤感", "伤感|日语", "摇滚|日语", "摇滚"} {
		for j := range 32 {
			a.roam.dealing[label] = append(a.roam.dealing[label], netease.Track{ID: int64(100*i + j + 1)})
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.pal = paletteFor(c.Theme())
		ui.Column(c).Children(func() {
			for _, label := range []string{"伤感", "摇滚", "日语"} {
				a.styleTile(c.Key(label), &roamStyle{label: label, glyph: label})
			}
		})
	}, 600, 600)
	for _, step := range []struct{ click, style, language string }{
		{"伤感", "伤感", ""},
		{"日语", "伤感", "日语"},
		{"摇滚", "摇滚", "日语"},
		{"日语", "摇滚", ""},
		{"摇滚", "", ""},
	} {
		tt.Frame()
		if err := tt.Click(step.click); err != nil {
			t.Fatal(err)
		}
		style, language := a.roamChoice()
		if style != step.style || language != step.language {
			t.Fatalf("after %s: %s / %s, want %s / %s", step.click, style, language, step.style, step.language)
		}
	}
	if a.radioStarting != "" {
		t.Fatal("deselecting the last tile left a station loading")
	}
	if got := styleWords("伤感|日语"); got != "伤感 日语" {
		t.Fatalf("combined playlist query: %q", got)
	}
}

// Ordinary FM remains clickable while roaming, even when clicking the last mode used.
func TestRoamReturnToOrdinaryFM(t *testing.T) {
	oldFM := personalFM
	calls := make(chan string, 1)
	personalFM = func(mode string) ([]netease.Track, error) { calls <- mode; return nil, errors.New("test feed") }
	defer func() { personalFM = oldFM }()
	a := newTestApp()
	a.settings = defaultSettings()
	a.fmMode = 2
	a.pl.Cue([]netease.Track{{ID: 1, Name: "current"}})
	a.pl.AttachRadio(styleRadio("伤感|日语"), func(func([]netease.Track)) {})
	a.route = route{page: pageFM}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.SetDark(true)
	tt.Frame()
	saveFixFrame(t, tt, "fm-roaming.png")
	if err := tt.Click("探索"); err != nil {
		t.Fatal(err)
	}
	if mode := wait(t, calls, "ordinary FM mode"); mode != netease.FMExplore {
		t.Fatalf("requested %q", mode)
	}
	if a.radioStarting != fmRadio(netease.FMExplore) {
		t.Fatalf("still roaming: %q", a.radioStarting)
	}
	tt.Frame()
	saveFixFrame(t, tt, "fm-ordinary.png")
	if style, language := a.roamChoice(); style != "" || language != "" {
		t.Fatal("style tiles remained selected")
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if mode := wait(t, calls, "keyboard FM mode"); mode != netease.FMFamiliar {
		t.Fatalf("keyboard requested %q", mode)
	}
}

// Save optional render evidence without turning appearance into brittle pixel assertions.
func saveFixFrame(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	if dir := os.Getenv("NM_FIX_FRAMES"); dir != "" {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// Deselecting a style that is already playing leaves its current song and ends the feed.
func TestRoamDeselectPlayingStyle(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "current"}, {ID: 2}})
	a.pl.AttachRadio(styleRadio("伤感"), func(func([]netease.Track)) {})
	tt := ui.NewTester(func(c *ui.Context) {
		a.pal = paletteFor(c.Theme())
		a.styleTile(c, &roamStyle{label: "伤感", glyph: "心"})
	}, 200, 150)
	tt.Frame()
	if err := tt.Click("伤感"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Radio() != "" || a.pl.Current().ID != 1 || len(a.pl.Queue()) != 1 || a.nowPlaying {
		t.Fatal("the second click did not leave the style and retain its current song")
	}
}

// A failed cover retries after backoff and becomes available without restarting.
func TestFixArtworkRetriesAfterFailure(t *testing.T) {
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	requests := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- struct{}{}
		if len(requests) == 1 {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		w.Write(pic.Bytes())
	}))
	defer server.Close()
	posted := make(chan func(), 4)
	c := newArtCache(func(fn func()) { posted <- fn })
	now := time.Unix(1e9, 0)
	c.now = func() time.Time { return now }
	c.get(server.URL, false)
	wait(t, posted, "failed cover")()
	if a := c.get(server.URL, false); a == nil || !a.failed || len(requests) != 1 {
		t.Fatal("failed cover did not back off")
	}
	now = now.Add(artRetry + time.Second)
	if c.get(server.URL, false) != nil {
		t.Fatal("expired failure was returned")
	}
	wait(t, posted, "retried cover")()
	if a := c.get(server.URL, false); a == nil || a.failed || a.bmp == nil || len(requests) != 2 {
		t.Fatal("cover was not recovered after retry")
	}
}
