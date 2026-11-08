package netease

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Uploading a song to the cloud drive goes as NetEase's own clients do it,
// and as the NeteaseCloudMusicApi project's "cloud" does: NetEase is asked
// whether it has the file already, by its MD5; if not, the file goes to its
// object storage (NOS) under a token; then the song is described and
// published to the drive.

// cloudBucket is where the object storage keeps the files of cloud drives.
const cloudBucket = "jd-musicrep-privatecloud-audio-public"

// cloudBitrate is the bitrate the clients claim for every upload: NetEase
// reads the file's own.
const cloudBitrate = "999000"

// CloudUploadExts are the kinds of audio file the cloud drive takes.
var CloudUploadExts = []string{"mp3", "flac", "m4a", "ogg", "wav", "ape", "aac", "wma"}

// CloudUploadMeta describes a song being uploaded, as its tags do: NetEase
// shows these where it matches the file to no song of its own.
type CloudUploadMeta struct {
	Title, Artist, Album string
}

// ErrCloudFull reports that the drive has no room left for the file.
var ErrCloudFull = errors.New("云盘空间不足")

// nosHosts answers which object storage hosts take uploads; tests replace
// it.
var nosHosts = func(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://wanproxy.127.net/lbs?version=1.0&bucketname="+cloudBucket, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var lbs struct {
		Upload []string `json:"upload"`
	}
	if err := json.NewDecoder(res.Body).Decode(&lbs); err != nil {
		return nil, err
	}
	if len(lbs.Upload) == 0 {
		return nil, errors.New("netease: no upload host")
	}
	return lbs.Upload, nil
}

// uploadClient sends files: as long as a large file takes, unlike the API's.
var uploadClient = &http.Client{}

// cloudCall calls the eapi, through eapiPostContext; tests replace it.
var cloudCall = eapiPostContext

// idString reads an ID NetEase gives as a number or as a string.
type idString string

func (s *idString) UnmarshalJSON(b []byte) error {
	*s = idString(strings.Trim(string(b), `"`))
	if *s == "null" {
		*s = ""
	}
	return nil
}

// cloudFileName is the name the object storage keeps a file under: its own
// without the extension, spaces or dots, as the clients send it.
func cloudFileName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.Join(strings.Fields(name), "")
	name = strings.ReplaceAll(name, ".", "_")
	if name == "" {
		name = "song"
	}
	return name
}

// cloudExt is the extension of path, in lower case and without the dot.
func cloudExt(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

// fileMD5 is the MD5 of the file at path, in hex.
func fileMD5(ctx context.Context, path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := md5.New()
	n, err := io.Copy(h, ctxReader{ctx, f})
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// ctxReader stops reading once ctx is done.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// progressReader reports how much of a body has been read.
type progressReader struct {
	ctx      context.Context
	r        io.Reader
	sent     int64
	total    int64
	progress func(sent, total int64)
	last     time.Time
}

func (r *progressReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	r.sent += int64(n)
	if r.progress != nil && (time.Since(r.last) > 100*time.Millisecond || r.sent == r.total) {
		r.last = time.Now()
		r.progress(r.sent, r.total)
	}
	return n, err
}

// cloudStatus fails unless NetEase answered code 200.
func cloudStatus(path string, code int, msg string) error {
	if code == 200 {
		return nil
	}
	if msg == "" {
		msg = "失败"
	}
	return fmt.Errorf("%s (%d)", msg, code)
}

// CloudUpload uploads the file at path to the listener's cloud drive, and
// says how far it has come with progress (nil for none) as it is sent. It
// returns the song ID the drive keeps it under.
func CloudUpload(ctx context.Context, path string, meta CloudUploadMeta, progress func(sent, total int64)) (int64, error) {
	ext := cloudExt(path)
	sum, size, err := fileMD5(ctx, path)
	if err != nil {
		return 0, err
	}
	if size == 0 {
		return 0, errors.New("文件是空的")
	}

	var check struct {
		Code       int      `json:"code"`
		Message    string   `json:"message"`
		Msg        string   `json:"msg"`
		NeedUpload bool     `json:"needUpload"`
		SongID     idString `json:"songId"`
	}
	if err := cloudCall(ctx, "/api/cloud/upload/check", map[string]any{
		"bitrate": cloudBitrate, "ext": "", "length": size, "md5": sum, "songId": "0", "version": 1,
	}, &check); err != nil {
		return 0, err
	}
	if err := cloudStatus("check", check.Code, check.Message+check.Msg); err != nil {
		if check.Code == 507 || strings.Contains(check.Message+check.Msg, "空间") {
			return 0, ErrCloudFull
		}
		return 0, err
	}

	var alloc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Result  struct {
			ObjectKey  string `json:"objectKey"`
			Token      string `json:"token"`
			ResourceID int64  `json:"resourceId"`
		} `json:"result"`
	}
	if err := cloudCall(ctx, "/api/nos/token/alloc", map[string]any{
		"bucket": cloudBucket, "ext": ext, "filename": cloudFileName(path), "local": false,
		"nos_product": 3, "type": "audio", "md5": sum,
	}, &alloc); err != nil {
		return 0, err
	}
	if err := cloudStatus("alloc", alloc.Code, alloc.Message+alloc.Msg); err != nil {
		return 0, err
	}

	if check.NeedUpload {
		if err := sendToNOS(ctx, path, size, sum, ext, alloc.Result.ObjectKey, alloc.Result.Token, progress); err != nil {
			return 0, err
		}
	} else if progress != nil {
		// NetEase has the file already: nothing to send.
		progress(size, size)
	}

	title, artist, album := meta.Title, meta.Artist, meta.Album
	if title == "" {
		title = cloudFileName(path)
	}
	if artist == "" {
		artist = "未知艺术家"
	}
	if album == "" {
		album = "未知专辑"
	}
	var info struct {
		Code    int      `json:"code"`
		Message string   `json:"message"`
		Msg     string   `json:"msg"`
		SongID  idString `json:"songId"`
	}
	// NetEase reads a file just sent before it describes it: until then it
	// answers 409, that it cannot read the audio, or takes too long to answer.
	for try := 0; ; try++ {
		info.Code, info.Message, info.Msg = 0, "", ""
		err := cloudCall(ctx, "/api/upload/cloud/info/v2", map[string]any{
			"md5": sum, "songid": string(check.SongID), "filename": filepath.Base(path),
			"song": title, "album": album, "artist": artist, "bitrate": cloudBitrate,
			"resourceId": alloc.Result.ResourceID,
		}, &info)
		if err == nil {
			err = cloudStatus("info", info.Code, info.Message+info.Msg)
		}
		if err == nil {
			break
		}
		if ctx.Err() != nil || try >= cloudInfoTries-1 || (info.Code != 409 && !isTimeout(err)) {
			return 0, err
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(cloudInfoWait):
		}
	}

	var pub struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := cloudCall(ctx, "/api/cloud/pub/v2", map[string]any{"songid": string(info.SongID)}, &pub); err != nil {
		return 0, err
	}
	// 201: the drive had the song already.
	if pub.Code != 201 {
		if err := cloudStatus("pub", pub.Code, pub.Message+pub.Msg); err != nil {
			return 0, err
		}
	}
	var id int64
	fmt.Sscan(string(info.SongID), &id)
	return id, nil
}

// sendToNOS sends the file to the object storage under the token.
func sendToNOS(ctx context.Context, path string, size int64, sum, ext, key, token string, progress func(sent, total int64)) error {
	hosts, err := nosHosts(ctx)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	body := &progressReader{ctx: ctx, r: f, total: size, progress: progress}
	// Only the first slash of the key is escaped, as the clients do: the
	// storage keeps the rest as the object's path.
	u := strings.TrimRight(hosts[0], "/") + "/" + cloudBucket + "/" + nosKey(key) + "?offset=0&complete=true&version=1.0"
	req, err := http.NewRequestWithContext(ctx, "POST", u, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("x-nos-token", token)
	req.Header.Set("Content-MD5", sum)
	// The clients send every kind of file as audio/mpeg; NetEase reads the
	// file itself.
	req.Header.Set("Content-Type", "audio/mpeg")
	res, err := uploadClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if nosDebug != nil {
		nosDebug(u, res.Status, string(b))
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("上传文件失败：%s %s", res.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

// cloudInfoTries and cloudInfoWait are how often, and how far apart, a song
// just sent is asked to be described; tests shorten the wait.
var (
	cloudInfoTries = 6
	cloudInfoWait  = 5 * time.Second
)

// isTimeout reports whether err is a request that took too long.
func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// nosDebug, when set, is told what the storage answered; live tests set it.
var nosDebug func(url, status, body string)

// nosKey is the object key as it goes in the storage's URL: its first slash
// escaped, the rest kept, each part escaped otherwise.
func nosKey(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	if len(parts) < 2 {
		return parts[0]
	}
	return parts[0] + "%2F" + strings.Join(parts[1:], "/")
}
