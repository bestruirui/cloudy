package local

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The songs in a folder are found by their suffixes, wherever they sit
// below it, and their tags and names describe them.
func TestScanReadsSongs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "album", "01 - 晴天.mp3"), taggedMP3(t))
	writeFile(t, filepath.Join(dir, "02 - 林俊杰 - 江南.mp3"), plainMP3())
	writeFile(t, filepath.Join(dir, "live.flac"), taggedFLAC())
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("not a song"))
	writeFile(t, filepath.Join(dir, "cover.jpg"), []byte("not a song"))

	songs, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 3 {
		t.Fatalf("found %d songs, want 3", len(songs))
	}
	byName := map[string]Song{}
	for _, s := range songs {
		byName[filepath.Base(s.Path)] = s
	}
	mp3, ok := byName["01 - 晴天.mp3"]
	if !ok {
		t.Fatal("the tagged MP3 is missing")
	}
	if mp3.Title != "晴天" || mp3.Artist != "周杰伦" || mp3.Album != "叶惠美" {
		t.Errorf("the MP3 reads %+v", mp3)
	}
	if mp3.Duration <= 0 {
		t.Error("the MP3 has no length")
	}
	if len(mp3.Picture) == 0 || mp3.PictureMIME != "image/jpeg" {
		t.Errorf("the MP3's cover is %q, %d bytes", mp3.PictureMIME, len(mp3.Picture))
	}

	fromName, ok := byName["02 - 林俊杰 - 江南.mp3"]
	if !ok {
		t.Fatal("the untagged MP3 is missing")
	}
	if fromName.Title != "江南" || fromName.Artist != "林俊杰" {
		t.Errorf("the untagged MP3 reads %+v", fromName)
	}

	flac, ok := byName["live.flac"]
	if !ok {
		t.Fatal("the FLAC is missing")
	}
	if flac.Title != "夜曲" || flac.Artist != "周杰伦" || flac.Album != "十一月的萧邦" {
		t.Errorf("the FLAC reads %+v", flac)
	}
	if flac.Duration != 2*time.Second {
		t.Errorf("the FLAC plays %v, want 2s", flac.Duration)
	}
	if len(flac.Picture) == 0 || flac.PictureMIME != "image/png" {
		t.Errorf("the FLAC's cover is %q, %d bytes", flac.PictureMIME, len(flac.Picture))
	}
}

// A file whose tags say nothing still plays, as its name describes it.
func TestReadWithoutTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "03 - 陈奕迅 - 十年.mp3")
	writeFile(t, path, plainMP3())
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "十年" || s.Artist != "陈奕迅" || s.Album != "" {
		t.Errorf("read %+v", s)
	}
	if s.Duration <= 0 {
		t.Error("no length")
	}
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// taggedMP3 is an MP3 with an ID3v2.3 tag: its title, artist, album and a
// front cover, then two frames of silence.
func taggedMP3(t *testing.T) []byte {
	t.Helper()
	frames := [][]byte{
		id3Frame("TIT2", append([]byte{3}, []byte("晴天")...)),
		id3Frame("TPE1", utf16Text("周杰伦")),
		id3Frame("TALB", append([]byte{3}, []byte("叶惠美")...)),
		id3Frame("APIC", append([]byte{3}, append([]byte("image/jpeg\x00\x03\x00"), []byte{0xff, 0xd8, 0xff, 0xd9}...)...)),
	}
	body := bytes.Join(frames, nil)
	header := []byte{'I', 'D', '3', 3, 0, 0, 0, 0, 0, 0}
	size := len(body)
	header[6], header[7], header[8], header[9] = byte(size>>21)&0x7f, byte(size>>14)&0x7f, byte(size>>7)&0x7f, byte(size)&0x7f
	return append(append(header, body...), mp3Frames()...)
}

// plainMP3 is two frames of silence, without a tag.
func plainMP3() []byte { return mp3Frames() }

func mp3Frames() []byte {
	frame := make([]byte, 417)
	copy(frame, []byte{0xff, 0xfb, 0x90, 0x00})
	return append(frame, frame...)
}

func id3Frame(id string, body []byte) []byte {
	b := make([]byte, 10+len(body))
	copy(b, id)
	binary.BigEndian.PutUint32(b[4:8], uint32(len(body)))
	copy(b[10:], body)
	return b
}

// utf16Text is a text frame's body in UTF-16, with the byte order mark
// Windows taggers write.
func utf16Text(s string) []byte {
	b := []byte{1, 0xff, 0xfe}
	for _, r := range s {
		b = binary.LittleEndian.AppendUint16(b, uint16(r))
	}
	return b
}

// taggedFLAC is a FLAC with its stream info, a comment and a front cover.
func taggedFLAC() []byte {
	info := make([]byte, 34)
	binary.BigEndian.PutUint16(info[0:2], 4096)
	binary.BigEndian.PutUint16(info[2:4], 4096)
	x := uint64(44100)<<44 | uint64(1)<<41 | uint64(15)<<36 | 88200
	binary.BigEndian.PutUint64(info[10:18], x)

	comments := [][2]string{{"TITLE", "夜曲"}, {"ARTIST", "周杰伦"}, {"ALBUM", "十一月的萧邦"}}
	var comment bytes.Buffer
	writeLE(&comment, uint32(len("test")))
	comment.WriteString("test")
	writeLE(&comment, uint32(len(comments)))
	for _, c := range comments {
		writeLE(&comment, uint32(len(c[0])+1+len(c[1])))
		comment.WriteString(c[0] + "=" + c[1])
	}

	var pic bytes.Buffer
	writeBE(&pic, 3) // front cover
	writeBE(&pic, uint32(len("image/png")))
	pic.WriteString("image/png")
	writeBE(&pic, 0) // no description
	writeBE(&pic, 1) // width, height, depth, colors
	writeBE(&pic, 1)
	writeBE(&pic, 8)
	writeBE(&pic, 0)
	cover := []byte{0x89, 'P', 'N', 'G'}
	writeBE(&pic, uint32(len(cover)))
	pic.Write(cover)

	b := []byte("fLaC")
	b = append(b, flacBlock(0, false, info)...)
	b = append(b, flacBlock(4, false, comment.Bytes())...)
	b = append(b, flacBlock(6, true, pic.Bytes())...)
	b = append(b, make([]byte, 16)...)
	return b
}

func flacBlock(typ byte, last bool, body []byte) []byte {
	b := make([]byte, 4+len(body))
	b[0] = typ
	if last {
		b[0] |= 0x80
	}
	b[1], b[2], b[3] = byte(len(body)>>16), byte(len(body)>>8), byte(len(body))
	copy(b[4:], body)
	return b
}

func writeLE(w *bytes.Buffer, v uint32) { binary.Write(w, binary.LittleEndian, v) }
func writeBE(w *bytes.Buffer, v uint32) { binary.Write(w, binary.BigEndian, v) }
