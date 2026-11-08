package netease

import (
	"encoding/json"
	"testing"
)

// A song of the cloud drive plays whole, under its file's names where
// NetEase matched it to none, and a taken-down song in the drive too.
func TestCloudItemTrack(t *testing.T) {
	var it cloudItem
	raw := `{"simpleSong":{"id":0,"name":"","ar":[{"id":0,"name":null}],"al":{"id":0,"name":null},"privilege":{"id":0,"st":-200,"pl":0}},
		"songId":1234,"songName":"我的录音","artist":"某人","album":"现场","fileName":"rec.flac"}`
	if err := json.Unmarshal([]byte(raw), &it); err != nil {
		t.Fatal(err)
	}
	tr := it.track()
	if tr.ID != 1234 || tr.Name != "我的录音" || tr.ArtistNames() != "某人" || tr.Album.Name != "现场" {
		t.Fatalf("track %+v", tr)
	}
	if !tr.Playable() {
		t.Fatal("a song of the drive is not playable")
	}
	taken := Track{ID: 1, Privilege: &Privilege{ID: 1, St: -200, Pl: 0, Cs: true}}
	if !taken.Playable() {
		t.Fatal("a taken-down song in the drive is not playable")
	}
	taken.Privilege.Cs = false
	if taken.Playable() {
		t.Fatal("a taken-down song plays")
	}
}

// The drive gives its sizes as numbers, and other accounts as strings.
func TestCloudSizes(t *testing.T) {
	for _, body := range []string{`{"size":1073741824,"maxSize":"64424509440"}`, `{"size":"1073741824","maxSize":64424509440}`} {
		var res struct {
			Size    json.Number `json:"size"`
			MaxSize json.Number `json:"maxSize"`
		}
		if err := json.Unmarshal([]byte(body), &res); err != nil {
			t.Fatal(body, err)
		}
		used, _ := res.Size.Int64()
		max, _ := res.MaxSize.Int64()
		if used != 1<<30 || max != 60<<30 {
			t.Errorf("%s: %d %d", body, used, max)
		}
	}
}
