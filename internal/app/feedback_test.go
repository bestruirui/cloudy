package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"netease-music-mygo/internal/netease"
)

// A message goes as a form, its images as the files of it, with what names
// the app, and an answer that it didn't go says why.
func TestFeedbackPost(t *testing.T) {
	var text, file string
	answer := "<html>Thanks!</html>"
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != feedbackFrom {
			t.Errorf("sent from %q", r.Header.Get("Referer"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		text = r.FormValue("反馈内容")
		if f, _, err := r.FormFile("attachment"); err == nil {
			b, _ := io.ReadAll(f)
			file = string(b)
		}
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	was := feedbackTo
	feedbackTo = srv.URL
	defer func() { feedbackTo = was }()

	path := filepath.Join(t.TempDir(), "shot.png")
	os.WriteFile(path, []byte("picture"), 0o644)
	if err := postFeedback("歌词错位了", "0.1.24", []string{path}); err != nil {
		t.Fatal(err)
	}
	if text != "歌词错位了" || file != "picture" {
		t.Errorf("got %q with %q", text, file)
	}
	status = http.StatusBadGateway
	if err := postFeedback("再一次", "0.1.24", nil); err == nil {
		t.Error("a refusal went unnoticed")
	}
	status = http.StatusOK
	// As FormSubmit answers the first message to a mailbox.
	answer = "<p>This form needs Activation. We've sent you an email containing an 'Activate Form' link.</p>"
	err := postFeedback("第一次", "0.1.24", nil)
	if !errors.Is(err, errFeedbackInactive) {
		t.Errorf("activation went unnoticed: %v", err)
	}
	if !strings.Contains(feedbackFailure(err), "激活") {
		t.Errorf("it says %q", feedbackFailure(err))
	}
}

// A message that goes leaves the card at once. One that comes back joins what
// has been written since, and the images with it.
func TestFeedbackComesBackIfItDidntGo(t *testing.T) {
	a := newTestApp()
	f := &a.feedback
	f.sending = true
	f.text = "刚写的"
	a.feedbackSent("先写的", []feedbackImage{{path: "a"}}, errors.New("down"))
	if f.sending || !f.failed || f.text != "先写的\n\n刚写的" || len(f.images) != 1 {
		t.Errorf("%+v", f)
	}
	f.text, f.images = "", nil
	f.sending = true
	a.feedbackSent("写的", nil, nil)
	if f.sending || f.failed || f.text != "" || a.toast == "" {
		t.Errorf("%+v", f)
	}
}

// Waiting too long for an answer says so, rather than that it failed.
func TestFeedbackTimeoutSaysSo(t *testing.T) {
	err := &url.Error{Op: "Post", URL: feedbackTo, Err: context.DeadlineExceeded}
	if got := feedbackFailure(err); !strings.Contains(got, "太慢") {
		t.Errorf("it says %q", got)
	}
}

// Images stop at what a message can hold, and say so.
func TestFeedbackLimits(t *testing.T) {
	a := newTestApp()
	a.addFeedbackImages([]feedbackImage{{path: "a", size: 6 << 20}, {path: "b", size: 6 << 20}}, false)
	if len(a.feedback.images) != 1 || !a.feedback.failed {
		t.Errorf("took %d images of 12 MB, failed %v", len(a.feedback.images), a.feedback.failed)
	}
	a = newTestApp()
	var many []feedbackImage
	for range feedbackImages + 2 {
		many = append(many, feedbackImage{path: "a", size: 1})
	}
	a.addFeedbackImages(many, false)
	if len(a.feedback.images) != feedbackImages {
		t.Errorf("took %d images", len(a.feedback.images))
	}
}

// A playlist's songs are found by what they are called and who sings them,
// sorted without touching the playlist, and its search ends with its page.
func TestPlaylistSearchAndSort(t *testing.T) {
	a := newTestApp()
	a.route = route{page: pagePlaylist, id: 1}
	song := func(id int64, name, singer string, ms int64) netease.Track {
		return netease.Track{ID: id, Name: name, Duration: ms, Artists: []netease.Artist{{Name: singer}}}
	}
	pl := &netease.Playlist{ID: 1, Tracks: []netease.Track{song(1, "晴天", "周杰伦", 200000), song(2, "Anchor", "Novo Amor", 300000), song(3, "稻香", "周杰伦", 100000)}}

	if got := a.listShown(pl.Tracks); len(got) != 3 {
		t.Fatalf("%d songs", len(got))
	}
	a.plView.query = "周杰伦"
	if got := a.listShown(pl.Tracks); len(got) != 2 {
		t.Errorf("found %d by their singer", len(got))
	}
	a.plView.query = "anchor"
	if got := a.listShown(pl.Tracks); len(got) != 1 || got[0].ID != 2 {
		t.Errorf("found %v, in another case", got)
	}
	a.plView.query, a.plView.sort = "", 3
	if got := a.listShown(pl.Tracks); got[0].ID != 2 || got[2].ID != 3 {
		t.Errorf("by length: %v", got)
	}
	if pl.Tracks[0].ID != 1 {
		t.Error("sorting moved the playlist's own songs")
	}
	a.route = route{page: pagePlaylist, id: 2}
	if a.listShown(pl.Tracks); a.plView.sort != 0 {
		t.Error("the next playlist opened sorted as the last one")
	}
}
