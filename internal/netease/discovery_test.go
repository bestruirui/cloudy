package netease

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type browseTransport func(*http.Request) (*http.Response, error)

func (f browseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// R&B keeps its ampersand in one category, and curated lists use their own endpoint.
func TestBrowsePlaylistRequests(t *testing.T) {
	old := httpClient
	defer func() { httpClient = old }()
	for _, tc := range []struct {
		name, path, cat string
		fetch           func() ([]Playlist, error)
	}{
		{"category", "/api/playlist/list", "R&B/Soul", func() ([]Playlist, error) { return CategoryPlaylists("R&B/Soul", 30) }},
		{"quality", "/api/playlist/highquality/list", "全部", func() ([]Playlist, error) { return HighQualityPlaylists(30) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient = &http.Client{Transport: browseTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "music.163.com" || r.URL.Path != tc.path || r.URL.Query().Get("cat") != tc.cat || r.URL.Query().Get("limit") != "30" {
					t.Fatalf("wrong playlist request: %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":200,"playlists":[{"id":42,"name":"playlist","coverImgUrl":"https://example.com/cover.jpg","playCount":12345}]}`)), Header: make(http.Header)}, nil
			})}
			lists, err := tc.fetch()
			if err != nil || len(lists) != 1 || lists[0].ID != 42 || lists[0].Cover() == "" || lists[0].PlayCount != 12345 {
				t.Fatalf("decoded playlists %+v: %v", lists, err)
			}
		})
	}
	httpClient = &http.Client{Transport: browseTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":503,"message":"unavailable"}`)), Header: make(http.Header)}, nil
	})}
	if _, err := CategoryPlaylists("学习", 30); err == nil {
		t.Fatal("an API failure became an empty successful category")
	}
}

// Live discovery reads all eight sources without a login or account writes.
func TestBrowsePlaylistsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip()
	}
	for _, category := range []string{"华语", "R&B/Soul", "轻音乐", "电子", "学习", "运动", "精品", "推荐"} {
		t.Run(category, func(t *testing.T) {
			var lists []Playlist
			var err error
			switch category {
			case "精品":
				lists, err = HighQualityPlaylists(3)
			case "推荐":
				lists, err = FeaturedPlaylists(3)
			default:
				lists, err = CategoryPlaylists(category, 3)
			}
			if err != nil || len(lists) == 0 {
				t.Fatalf("no playlists: %v", err)
			}
			if lists[0].ID == 0 || lists[0].Cover() == "" {
				t.Fatalf("missing playlist identity or artwork: %+v", lists[0])
			}
			t.Logf("%d playlists; first %d %s", len(lists), lists[0].ID, lists[0].Name)
		})
	}
}
