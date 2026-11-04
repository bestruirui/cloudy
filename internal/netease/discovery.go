package netease

import (
	"net/url"
	"strconv"
)

// CategoryPlaylists returns the most popular playlists tagged category by NetEase.
func CategoryPlaylists(category string, limit int) ([]Playlist, error) {
	var res struct {
		Playlists []Playlist `json:"playlists"`
	}
	err := get("/api/playlist/list", url.Values{
		"cat": {category}, "order": {"hot"}, "limit": {strconv.Itoa(limit)},
		"offset": {"0"}, "total": {"true"},
	}, &res)
	return res.Playlists, err
}

// HighQualityPlaylists returns NetEase's curated playlists, across all categories.
func HighQualityPlaylists(limit int) ([]Playlist, error) {
	var res struct {
		Playlists []Playlist `json:"playlists"`
	}
	err := get("/api/playlist/highquality/list", url.Values{
		"cat": {"全部"}, "limit": {strconv.Itoa(limit)}, "lasttime": {"0"},
	}, &res)
	return res.Playlists, err
}
