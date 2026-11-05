package player

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"netease-music-mygo/internal/netease"
)

// Uncached songs used to retain their entire download in memory.
func TestUncachedDownloadUsesTemporaryFile(t *testing.T) {
	body := bytes.Repeat([]byte("audio"), 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer srv.Close()
	for _, cache := range []*Cache{nil, NewCache(t.TempDir(), 0), NewCache(t.TempDir(), 1)} {
		d := cache.download(7, &netease.Stream{URL: srv.URL, Size: int64(len(body)), Level: "exhigh", Type: "mp3"})
		defer d.Close()
		d.mu.Lock()
		path, file := d.part, d.file
		d.mu.Unlock()
		if file == nil || path == "" {
			t.Fatal("uncached audio was not spooled to a file")
		}
		got, err := io.ReadAll(&downloadReader{d: d})
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("download: %v, %d bytes", err, len(got))
		}
		d.mu.Lock()
		memory := len(d.data)
		d.mu.Unlock()
		if memory != 0 {
			t.Fatal("the spool retained a copy in memory")
		}
		// Wait for the writer's release as well as EOF before the last reader closes.
		d.mu.Lock()
		for d.users > 1 {
			d.mu.Unlock()
			runtime.Gosched()
			d.mu.Lock()
		}
		d.mu.Unlock()
		d.Close()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("the spool survived its last reader")
		}
	}
}

// A failed spool must not let a long download grow beyond its memory budget.
func TestDownloadMemoryBound(t *testing.T) {
	d := downloaded(nil)
	d.n = maxMemoryDownload
	if err := d.write([]byte{1}); !errors.Is(err, errDownloadMemory) {
		t.Fatalf("buffer overflow: %v", err)
	}
	if len(d.data) != 0 || d.n != maxMemoryDownload {
		t.Fatal("overflow changed the buffered data")
	}
}
