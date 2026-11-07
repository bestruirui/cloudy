package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// tempHistory keeps the songs played in a file of the test's own.
func tempHistory(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "history.json")
	old := historyPath
	historyPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { historyPath = old })
	return path
}

func song(id int64, name string) netease.Track {
	return netease.Track{ID: id, Name: name, Duration: 200000}
}

// A song played again moves to the head, once, and the list keeps to its
// limit; a file played tells apart from another by its path.
func TestAddPlayed(t *testing.T) {
	now := time.Now()
	var played []playedSong
	played = addPlayed(played, song(1, "a"), now)
	played = addPlayed(played, song(2, "b"), now)
	played = addPlayed(played, song(1, "a"), now)
	if len(played) != 2 || played[0].Track.ID != 1 || played[1].Track.ID != 2 {
		t.Fatalf("played: %+v", played)
	}
	played = addPlayed(played, netease.Track{Name: "x", LocalPath: `D:\x.flac`}, now)
	played = addPlayed(played, netease.Track{Name: "y", LocalPath: `D:\y.flac`}, now)
	if len(played) != 4 {
		t.Fatalf("two files played came in as %d songs", len(played))
	}
	for i := range historyLimit + 20 {
		played = addPlayed(played, song(int64(100+i), "n"), now)
	}
	if len(played) != historyLimit || played[0].Track.ID != int64(100+historyLimit+19) {
		t.Fatalf("kept %d, the latest %d", len(played), played[0].Track.ID)
	}
}

// A song counts once heard for a while, or to its end, and is kept in the
// file for the next time the app opens.
func TestRememberKeepsTheSongsPlayed(t *testing.T) {
	path := tempHistory(t)
	a := newTestApp()
	a.recent.loaded = false
	a.remember(song(1, "skipped"), 3*time.Second, false)
	a.remember(song(2, "heard"), historyAfter, false)
	a.remember(song(3, "short, to its end"), 5*time.Second, true)
	if len(a.recent.played) != 2 || a.recent.played[0].Track.ID != 3 || a.recent.played[1].Track.ID != 2 {
		t.Fatalf("played: %+v", a.recent.played)
	}
	// The file is written off the main thread.
	deadline := time.Now().Add(3 * time.Second)
	for {
		b := newTestApp()
		b.recent.loaded = false
		b.loadHistory()
		if len(b.recent.played) == 2 && b.recent.played[0].Track.Name == "short, to its end" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the file has %+v", b.recent.played)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Error("a part file was left")
	}
}

// 最近播放 lists the song playing, then those played on this computer, and
// needs no account; 清空记录 asks again before it clears.
func TestRecentPageIsThisComputers(t *testing.T) {
	tempHistory(t)
	a := newTestApp()
	a.settings = defaultSettings()
	a.recent.played = []playedSong{{Track: song(2, "昨天听的")}, {Track: song(3, "前天听的")}}
	a.route = route{page: pageRecent}
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	if !tt.HasText("昨天听的") || !tt.HasText("在这台电脑上用 Cloudy 听过的 2 首") {
		t.Fatalf("the page shows %v", tt.Texts())
	}
	if err := tt.Click("清空记录"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.recent.played) != 2 || !tt.HasText("再点一次清空") {
		t.Fatal("one click cleared the songs played")
	}
	if err := tt.Click("再点一次清空"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(a.recent.played) != 0 || !tt.HasText("还没有播放记录：在 Cloudy 里听过的歌会出现在这里") {
		t.Errorf("cleared, the page shows %v", tt.Texts())
	}
}
