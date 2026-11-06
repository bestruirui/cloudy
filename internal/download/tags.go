package download

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Tags are what a downloaded song's file says of it, so that it shows as
// itself in other players, and here again if the index is lost.
type Tags struct {
	Title, Artist, Album string
	Lyrics               string // as LRC
	Cover                []byte // JPEG or PNG
}

// WriteTags writes tags into the MP3 or FLAC file at path, in place of
// those it has: an ID3v2.3 tag, or FLAC's comments and picture. The file
// is written beside it and moved over it, so a failure leaves it whole.
func WriteTags(path string, t Tags) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var out []byte
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		out = tagMP3(src, t)
	case ".flac":
		if out, err = tagFLAC(src, t); err != nil {
			return err
		}
	default:
		return errors.New("download: not an MP3 or a FLAC")
	}
	tmp := path + ".tagging"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// LyricsPath is where the .lrc of the song at path is kept: beside it, by
// the same name, as players look for it.
func LyricsPath(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".lrc"
}

// coverMIME says what the picture b is.
func coverMIME(b []byte) string {
	if bytes.HasPrefix(b, []byte("\x89PNG")) {
		return "image/png"
	}
	return "image/jpeg"
}

// id3Len returns the length of the ID3v2 tag at the start of b, or 0.
func id3Len(b []byte) int {
	if len(b) < 10 || string(b[:3]) != "ID3" {
		return 0
	}
	n := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	n += 10
	if b[5]&0x10 != 0 {
		n += 10 // a footer
	}
	return min(n, len(b))
}

// tagMP3 returns src with an ID3v2.3 tag of t in place of the one it had.
func tagMP3(src []byte, t Tags) []byte {
	var frames bytes.Buffer
	text := func(id, s string) {
		if s == "" {
			return
		}
		frame(&frames, id, append([]byte{1}, utf16BOM(s)...))
	}
	text("TIT2", t.Title)
	text("TPE1", t.Artist)
	text("TALB", t.Album)
	if t.Lyrics != "" {
		// Encoding, language, an empty description, the text.
		body := append([]byte{1}, "XXX"...)
		body = append(body, utf16BOM("")...)
		body = append(body, utf16BOM(t.Lyrics)...)
		frame(&frames, "USLT", body)
	}
	if len(t.Cover) > 0 {
		// Encoding, MIME type, the front cover, no description, the picture.
		body := append([]byte{0}, coverMIME(t.Cover)...)
		body = append(body, 0, 3, 0)
		body = append(body, t.Cover...)
		frame(&frames, "APIC", body)
	}
	size := frames.Len()
	head := []byte{'I', 'D', '3', 3, 0, 0, byte(size >> 21 & 0x7f), byte(size >> 14 & 0x7f), byte(size >> 7 & 0x7f), byte(size & 0x7f)}
	out := make([]byte, 0, 10+size+len(src))
	out = append(out, head...)
	out = append(out, frames.Bytes()...)
	return append(out, src[id3Len(src):]...)
}

// frame adds an ID3v2.3 frame: its ID, its size, no flags, its body.
func frame(w *bytes.Buffer, id string, body []byte) {
	w.WriteString(id)
	binary.Write(w, binary.BigEndian, uint32(len(body)))
	w.Write([]byte{0, 0})
	w.Write(body)
}

// utf16BOM is s in UTF-16 with its byte order mark, ended by a zero.
func utf16BOM(s string) []byte {
	out := []byte{0xff, 0xfe}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u), byte(u>>8))
	}
	return append(out, 0, 0)
}

// tagFLAC returns src with comments and a picture of t in place of those
// it had, its other metadata as it was.
func tagFLAC(src []byte, t Tags) ([]byte, error) {
	off := id3Len(src)
	if len(src) < off+4 || string(src[off:off+4]) != "fLaC" {
		return nil, errors.New("download: not a FLAC file")
	}
	p := off + 4
	var keep [][]byte // whole blocks, header and all
	var vendor []byte
	for {
		if p+4 > len(src) {
			return nil, io.ErrUnexpectedEOF
		}
		last := src[p]&0x80 != 0
		typ := src[p] & 0x7f
		n := int(src[p+1])<<16 | int(src[p+2])<<8 | int(src[p+3])
		if p+4+n > len(src) {
			return nil, io.ErrUnexpectedEOF
		}
		block := src[p : p+4+n]
		switch typ {
		case 4: // the comments: their vendor stays
			if n >= 4 {
				vl := int(binary.LittleEndian.Uint32(block[4:8]))
				if 8+vl <= len(block) {
					vendor = block[8 : 8+vl]
				}
			}
		case 6, 1: // pictures and padding go
		default:
			keep = append(keep, block)
		}
		p += 4 + n
		if last {
			break
		}
	}
	if len(keep) == 0 || keep[0][0]&0x7f != 0 {
		return nil, errors.New("download: FLAC without stream info")
	}
	if vendor == nil {
		vendor = []byte("Cloudy")
	}
	var comments bytes.Buffer
	binary.Write(&comments, binary.LittleEndian, uint32(len(vendor)))
	comments.Write(vendor)
	var fields []string
	add := func(k, v string) {
		if v != "" {
			fields = append(fields, k+"="+v)
		}
	}
	add("TITLE", t.Title)
	add("ARTIST", t.Artist)
	add("ALBUM", t.Album)
	add("LYRICS", t.Lyrics)
	binary.Write(&comments, binary.LittleEndian, uint32(len(fields)))
	for _, f := range fields {
		binary.Write(&comments, binary.LittleEndian, uint32(len(f)))
		comments.WriteString(f)
	}
	blocks := append([][]byte(nil), keep...)
	blocks = append(blocks, flacBlock(4, comments.Bytes()))
	if len(t.Cover) > 0 {
		var pic bytes.Buffer
		mime := coverMIME(t.Cover)
		binary.Write(&pic, binary.BigEndian, uint32(3)) // the front cover
		binary.Write(&pic, binary.BigEndian, uint32(len(mime)))
		pic.WriteString(mime)
		binary.Write(&pic, binary.BigEndian, uint32(0)) // no description
		pic.Write(make([]byte, 16))                      // size and colors, unknown
		binary.Write(&pic, binary.BigEndian, uint32(len(t.Cover)))
		pic.Write(t.Cover)
		if pic.Len() < 1<<24 {
			blocks = append(blocks, flacBlock(6, pic.Bytes()))
		}
	}
	out := make([]byte, 0, len(src)+len(t.Cover)+len(t.Lyrics)+1024)
	out = append(out, "fLaC"...)
	for i, b := range blocks {
		h := b[0] & 0x7f
		if i == len(blocks)-1 {
			h |= 0x80
		}
		out = append(out, h)
		out = append(out, b[1:]...)
	}
	return append(out, src[p:]...), nil
}

// flacBlock is a metadata block of type typ holding body.
func flacBlock(typ byte, body []byte) []byte {
	n := len(body)
	return append([]byte{typ, byte(n >> 16), byte(n >> 8), byte(n)}, body...)
}
