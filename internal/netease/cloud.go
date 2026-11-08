package netease

import (
	"encoding/json"
	"net/url"
	"strconv"
)

// CloudDrive is the listener's music cloud drive: the songs they uploaded
// to NetEase, and how much of its space they take.
type CloudDrive struct {
	Songs []Track
	// Used and Max are the bytes the songs take and may take.
	Used, Max int64
}

// cloudPage is how many songs of the drive one request asks for.
const cloudPage = 200

// cloudItem is a song of the drive: the song NetEase matched it to, or its
// own when it matched none, and what the file says of it.
type cloudItem struct {
	Song     Track  `json:"simpleSong"`
	SongID   int64  `json:"songId"`
	SongName string `json:"songName"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	FileName string `json:"fileName"`
}

// track is the drive's song as one to play: one the listener owns, and so
// plays whole whatever its rights, under the names of its file where
// NetEase knows none.
func (it cloudItem) track() Track {
	t := it.Song
	if t.ID == 0 {
		t.ID = it.SongID
	}
	if t.Name == "" {
		t.Name = it.SongName
	}
	if t.Name == "" {
		t.Name = it.FileName
	}
	if len(t.Artists) == 0 || t.Artists[0].Name == "" {
		if it.Artist != "" {
			t.Artists = []Artist{{Name: it.Artist}}
		}
	}
	if t.Album.Name == "" {
		t.Album.Name = it.Album
	}
	pl := 320000
	if t.Privilege != nil && t.Privilege.Pl > pl {
		pl = t.Privilege.Pl
	}
	t.Privilege = &Privilege{ID: t.ID, Pl: pl, Cs: true}
	return t
}

// CloudSongs returns the songs of the listener's cloud drive, those
// uploaded last first.
func CloudSongs() (*CloudDrive, error) {
	drive := &CloudDrive{}
	for offset := 0; ; offset += cloudPage {
		var res struct {
			Data    []cloudItem `json:"data"`
			Size    json.Number `json:"size"`
			MaxSize json.Number `json:"maxSize"`
			HasMore bool        `json:"hasMore"`
		}
		q := url.Values{"limit": {strconv.Itoa(cloudPage)}, "offset": {strconv.Itoa(offset)}}
		if err := get("/api/v1/cloud/get", q, &res); err != nil {
			return nil, err
		}
		drive.Used, _ = res.Size.Int64()
		drive.Max, _ = res.MaxSize.Int64()
		for _, it := range res.Data {
			drive.Songs = append(drive.Songs, it.track())
		}
		if !res.HasMore || len(res.Data) == 0 {
			return drive, nil
		}
	}
}
