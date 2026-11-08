package netease

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An upload asks whether NetEase has the file, sends it to the object
// storage under a token when not, then describes and publishes it.
func TestCloudUpload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "My Song.v2.mp3")
	os.WriteFile(path, []byte("not really audio, but bytes"), 0o600)

	var sent []byte
	var header http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header
		if !strings.Contains(r.RequestURI, "/obj%2Fa/b.data?") {
			t.Errorf("object key sent as %s", r.RequestURI)
		}
		if !strings.HasPrefix(r.URL.Path, "/"+cloudBucket+"/") || r.URL.Query().Get("complete") != "true" {
			t.Errorf("sent to %s", r.URL)
		}
		sent, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"offset":27}`))
	}))
	defer srv.Close()

	var calls []string
	var info map[string]any
	oldCall, oldHosts, oldWait := cloudCall, nosHosts, cloudInfoWait
	defer func() { cloudCall, nosHosts, cloudInfoWait = oldCall, oldHosts, oldWait }()
	cloudInfoWait = 0
	nosHosts = func(context.Context) ([]string, error) { return []string{srv.URL}, nil }
	cloudCall = func(ctx context.Context, path string, data map[string]any, out any) error {
		calls = append(calls, path)
		body := `{"code":200}`
		switch path {
		case "/api/cloud/upload/check":
			body = `{"code":200,"needUpload":true,"songId":"ABC"}`
		case "/api/nos/token/alloc":
			if data["filename"] != "MySong_v2" || data["ext"] != "mp3" {
				t.Errorf("alloc %v", data)
			}
			body = `{"code":200,"result":{"objectKey":"obj/a/b.data","token":"tok","resourceId":84160185246}}`
		case "/api/upload/cloud/info/v2":
			// Not read yet the first time.
			if info == nil {
				info = data
				body = `{"code":409,"message":"上传文件的音频解析失败"}`
				break
			}
			info = data
			body = `{"code":200,"songId":"123"}`
		case "/api/cloud/pub/v2":
			if data["songid"] != "123" {
				t.Errorf("pub %v", data)
			}
		}
		return jsonDecode(body, out)
	}

	var last int64
	id, err := CloudUpload(context.Background(), path, CloudUploadMeta{Artist: "歌手"}, func(s, _ int64) { last = s })
	if err != nil {
		t.Fatal(err)
	}
	if id != 123 || string(sent) != "not really audio, but bytes" || last != int64(len(sent)) {
		t.Fatalf("id %d sent %q progress %d", id, sent, last)
	}
	if header.Get("x-nos-token") != "tok" || header.Get("Content-MD5") == "" {
		t.Errorf("headers %v", header)
	}
	if len(calls) != 5 {
		t.Errorf("calls %v", calls)
	}
	if info["song"] != "MySong_v2" || info["artist"] != "歌手" || info["album"] != "未知专辑" || info["songid"] != "ABC" || info["resourceId"] != int64(84160185246) || info["objectKey"] != nil {
		t.Errorf("info %v", info)
	}
}

// A file NetEase has already is not sent again.
func TestCloudUploadKnownFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.flac")
	os.WriteFile(path, []byte("x"), 0o600)
	oldCall, oldHosts := cloudCall, nosHosts
	defer func() { cloudCall, nosHosts = oldCall, oldHosts }()
	nosHosts = func(context.Context) ([]string, error) {
		t.Error("sent a file NetEase has")
		return nil, nil
	}
	cloudCall = func(ctx context.Context, path string, data map[string]any, out any) error {
		switch path {
		case "/api/cloud/upload/check":
			return jsonDecode(`{"code":200,"needUpload":false,"songId":"9"}`, out)
		case "/api/upload/cloud/info/v2":
			return jsonDecode(`{"code":200,"songId":9}`, out)
		case "/api/cloud/pub/v2":
			return jsonDecode(`{"code":201}`, out)
		}
		return jsonDecode(`{"code":200,"result":{}}`, out)
	}
	if id, err := CloudUpload(context.Background(), path, CloudUploadMeta{Title: "a"}, nil); err != nil || id != 9 {
		t.Fatal(id, err)
	}
}

func jsonDecode(s string, out any) error { return json.Unmarshal([]byte(s), out) }
