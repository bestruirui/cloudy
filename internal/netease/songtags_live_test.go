package netease

import (
	"os"
	"testing"
)

// A well-known song has its genre and its mood in NetEase's wiki.
func TestSongTagsLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("LIVE unset")
	}
	genres, moods, err := SongTags(186016)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("genres %q moods %q", genres, moods)
	if len(genres) == 0 {
		t.Error("no genre")
	}
}
