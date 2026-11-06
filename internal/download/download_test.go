package download

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"netease-music-mygo/internal/netease"
)

func testTrack() netease.Track {
	return netease.Track{
		ID:      2652820720,
		Name:    "晴天",
		Artists: []netease.Artist{{Name: "测试歌手"}},
		Album:   netease.Album{Name: "叶惠美"},
	}
}

// A song arrives as a file named for it, with its bytes and size kept.
func TestFetchSavesSong(t *testing.T) {
	old := songURL
	defer func() { songURL = old }()
	payload := append([]byte("ID3"), bytes.Repeat([]byte{0x55}, 4096)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://music.163.com/" {
			t.Errorf("referer %q", r.Header.Get("Referer"))
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Write(payload)
	}))
	defer srv.Close()
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		return &netease.Stream{URL: srv.URL + "/song.mp3", Type: "mp3", Level: "exhigh", Size: int64(len(payload))}, nil
	}
	dir := t.TempDir()
	var lastDone, lastTotal int64
	item, err := Fetch(context.Background(), testTrack(), "exhigh", dir, func(done, total int64) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != filepath.Join(dir, "测试歌手 - 晴天.mp3") {
		t.Fatalf("path %q", item.Path)
	}
	if item.Size != int64(len(payload)) || item.Level != "exhigh" {
		t.Fatalf("item %+v", item)
	}
	b, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, payload) {
		t.Error("the file is not what was served")
	}
	if lastDone != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Errorf("progress %d / %d", lastDone, lastTotal)
	}
}

// A lossless stream is kept as FLAC, under that extension.
func TestFetchSavesFLAC(t *testing.T) {
	old := songURL
	defer func() { songURL = old }()
	payload := append([]byte("fLaC"), bytes.Repeat([]byte{0x77}, 2048)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		return &netease.Stream{URL: srv.URL + "/song.flac", Type: "flac", Level: "lossless", Size: int64(len(payload))}, nil
	}
	item, err := Fetch(context.Background(), testTrack(), "lossless", t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(item.Path, ".flac") || item.Level != "lossless" {
		t.Fatalf("item %+v", item)
	}
	b, err := os.ReadFile(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, payload) {
		t.Error("the FLAC is not what was served")
	}
}

// A second song of the same name does not write over the first.
func TestFetchKeepsBothOfTheSameName(t *testing.T) {
	old := songURL
	defer func() { songURL = old }()
	payload := append([]byte("ID3"), bytes.Repeat([]byte{0x66}, 128)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		return &netease.Stream{URL: srv.URL + "/song.mp3", Type: "mp3", Level: "exhigh"}, nil
	}
	dir := t.TempDir()
	first, err := Fetch(context.Background(), testTrack(), "exhigh", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Fetch(context.Background(), testTrack(), "exhigh", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path == second.Path || !strings.Contains(second.Path, " (2).mp3") {
		t.Fatalf("paths %q and %q", first.Path, second.Path)
	}
	for _, path := range []string{first.Path, second.Path} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// A page served where the song should be is not kept as a song.
func TestFetchRejectsWhatIsNotAudio(t *testing.T) {
	old := songURL
	defer func() { songURL = old }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<!doctype html><title>not a song</title>"))
	}))
	defer srv.Close()
	songURL = func(ctx context.Context, id int64, level string) (*netease.Stream, error) {
		return &netease.Stream{URL: srv.URL + "/song.mp3", Type: "mp3", Level: "exhigh"}, nil
	}
	dir := t.TempDir()
	if _, err := Fetch(context.Background(), testTrack(), "exhigh", dir, nil); err == nil {
		t.Fatal("kept a page as a song")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("left %d files behind", len(entries))
	}
}

// A name a file system refuses has those characters taken out.
func TestFileName(t *testing.T) {
	tr := testTrack()
	tr.Name = "晴:天?"
	tr.Artists = []netease.Artist{{Name: "A/B"}}
	if got, want := FileName(tr, "flac"), "A-B - 晴-天-.flac"; got != want {
		t.Errorf("name %q, want %q", got, want)
	}
	tr.Name, tr.Artists = "", nil
	if got, want := FileName(tr, "mp3"), "未知歌手 - 未知歌曲.mp3"; got != want {
		t.Errorf("name %q, want %q", got, want)
	}
	tr.Name, tr.Artists = "晴天", nil
	if got, want := FileName(tr, "mp3"), "未知歌手 - 晴天.mp3"; got != want {
		t.Errorf("name %q, want %q", got, want)
	}
}

// The index comes back as it was written, and a missing one is no songs.
func TestStoreRoundTrip(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "downloads.json")}
	if items, err := store.Load(); err != nil || len(items) != 0 {
		t.Fatalf("missing: %v, %d", err, len(items))
	}
	items := []Item{{
		Track: testTrack(),
		Path:  filepath.Join("C:", "Music", "晴天.mp3"),
		Level: "exhigh",
		Size:  1234,
		At:    time.Date(2026, 11, 6, 9, 30, 0, 0, time.UTC),
	}}
	if err := store.Save(items); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Track.Name != "晴天" || got[0].Path != items[0].Path || got[0].Size != 1234 {
		t.Fatalf("loaded %+v", got)
	}
}
