package download

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"netease-music-mygo/internal/local"
)

// A downloaded song's file carries its title, artist, album and cover, and
// still starts as the audio it was, MP3 or FLAC.
func TestWriteTags(t *testing.T) {
	cover := append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{7}, 300)...)
	for _, name := range []string{"seek.mp3", "seek.flac"} {
		src, err := os.ReadFile(filepath.Join("..", "player", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "歌手 - 歌名"+filepath.Ext(name))
		os.WriteFile(path, src, 0o644)
		tags := Tags{Title: "晴天", Artist: "周杰伦", Album: "叶惠美", Lyrics: "[00:01.00]故事的小黄花", Cover: cover}
		if err := WriteTags(path, tags); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// Twice: the tag written first is replaced, not stacked.
		if err := WriteTags(path, tags); err != nil {
			t.Fatalf("%s again: %v", name, err)
		}
		s, err := local.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if s.Title != "晴天" || s.Artist != "周杰伦" || s.Album != "叶惠美" {
			t.Errorf("%s: read %q / %q / %q", name, s.Title, s.Artist, s.Album)
		}
		if !bytes.Equal(s.Picture, cover) {
			t.Errorf("%s: cover of %d bytes, want %d", name, len(s.Picture), len(cover))
		}
		got, _ := os.ReadFile(path)
		if !bytes.HasSuffix(got, src[id3Len(src)+audioStart(t, src[id3Len(src):], name):]) {
			t.Errorf("%s: the audio changed", name)
		}
	}
}

// audioStart is where the audio of b starts after FLAC's metadata, or 0.
func audioStart(t *testing.T, b []byte, name string) int {
	if filepath.Ext(name) != ".flac" {
		return 0
	}
	p := 4
	for {
		last := b[p]&0x80 != 0
		p += 4 + (int(b[p+1])<<16 | int(b[p+2])<<8 | int(b[p+3]))
		if last {
			return p
		}
	}
}
