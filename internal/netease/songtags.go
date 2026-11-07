package netease

import "strconv"

// SongTags returns what NetEase's song wiki says a song is: its genres, as
// "流行-华语流行", and the tags of its mood and theme, as "思念". Either may
// be empty: many songs have neither.
func SongTags(id int64) (genres, moods []string, err error) {
	type element struct {
		MainTitle *struct {
			Title string `json:"title"`
		} `json:"mainTitle"`
	}
	var res struct {
		Code int `json:"code"`
		Data struct {
			Blocks []struct {
				Code      string `json:"code"`
				Creatives []struct {
					CreativeType string `json:"creativeType"`
					Resources    []struct {
						UIElement element `json:"uiElement"`
					} `json:"resources"`
				} `json:"creatives"`
			} `json:"blocks"`
		} `json:"data"`
	}
	if err := eapiPost("/api/song/play/about/block/page", map[string]any{"songId": strconv.FormatInt(id, 10)}, &res); err != nil {
		return nil, nil, err
	}
	for _, b := range res.Data.Blocks {
		if b.Code != "SONG_PLAY_ABOUT_SONG_BASIC" {
			continue
		}
		for _, c := range b.Creatives {
			for _, r := range c.Resources {
				t := r.UIElement.MainTitle
				if t == nil || t.Title == "" {
					continue
				}
				switch c.CreativeType {
				case "songTag":
					genres = append(genres, t.Title)
				case "songBizTag":
					moods = append(moods, t.Title)
				}
			}
		}
	}
	return genres, moods, nil
}
