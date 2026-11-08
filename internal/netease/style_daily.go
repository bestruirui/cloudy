package netease

import (
	"encoding/json"
	"fmt"
)

// The day's songs in a style: the official client's 风格推荐. The listener
// picks a few tags of one category, kept with the account, and NetEase picks
// the day's songs in them.

// A StyleTag is a style the songs can be picked in, as R&B or 治愈.
type StyleTag struct {
	ID       int64  `json:"tagId"`
	Name     string `json:"tagName"`
	Category int64  `json:"categoryId"`
}

// A StyleCategory is a kind of style: 曲风, 情绪, 场景, 语种 or 主题.
type StyleCategory struct {
	ID   int64      `json:"categoryId"`
	Name string     `json:"categoryName"`
	Tags []StyleTag `json:"tagVOList"`
}

// StyleSongs is the day's songs in the styles the listener picked.
type StyleSongs struct {
	Picked StyleCategory // the category, with the tags picked in it
	Songs  []Track
}

// StyleCategories returns the styles the songs can be picked in.
func StyleCategories() ([]StyleCategory, error) {
	var res struct {
		apiError
		Data struct {
			Categories []StyleCategory `json:"categorys"`
		} `json:"data"`
	}
	path := "/api/homepage/daily/song/config/get"
	if err := eapiPost(path, map[string]any{}, &res); err != nil {
		return nil, err
	}
	if err := res.check(path); err != nil {
		return nil, err
	}
	return res.Data.Categories, nil
}

// DailyStyleSongs returns the day's songs in the styles the listener picked,
// and which those are.
func DailyStyleSongs() (StyleSongs, error) {
	var res struct {
		apiError
		Data struct {
			Tags       StyleCategory `json:"tags"`
			DailySongs []Track       `json:"dailySongs"`
		} `json:"data"`
	}
	path := "/api/homepage/category/daily/song/list"
	if err := eapiPost(path, map[string]any{}, &res); err != nil {
		return StyleSongs{}, err
	}
	if err := res.check(path); err != nil {
		return StyleSongs{}, err
	}
	// They come without privileges, as the daily songs do.
	songs := res.Data.DailySongs
	detailPrivileges(songs)
	return StyleSongs{Picked: res.Data.Tags, Songs: songs}, nil
}

// SaveStyleTags keeps tags of category as the styles the listener's songs
// are picked in, with the account: the official client picks in them too.
func SaveStyleTags(category int64, tags []int64) error {
	b, err := json.Marshal(map[string]any{"tagIds": tags, "categoryId": category})
	if err != nil {
		return err
	}
	return eapiOK("/api/homepage/daily/song/tag/save", map[string]any{"tags": string(b)})
}

// check fails unless NetEase answered code 200.
func (e apiError) check(path string) error {
	if e.Code == 200 {
		return nil
	}
	msg := e.Message
	if msg == "" {
		msg = e.Msg
	}
	return fmt.Errorf("netease: %s: code %d %s", path, e.Code, msg)
}
