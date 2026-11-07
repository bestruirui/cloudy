package netease

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Scrobble tells NetEase that the listener signed in played the song id,
// from the playlist or album source if known, for seconds: it counts in
// their listening, as plays of NetEase's own clients do. ended says it
// played to its end, rather than the listener moving on.
//
// This is the log NetEase's web player sends, which it does not document:
// the listener turns it on knowing that.
func Scrobble(id, source int64, seconds int, ended bool) error {
	end := "ui"
	if ended {
		end = "playend"
	}
	entry := map[string]any{
		"action": "play",
		"json": map[string]any{
			"download": 0,
			"end":      end,
			"id":       id,
			"sourceId": strconv.FormatInt(source, 10),
			"time":     seconds,
			"type":     "song",
			"wifi":     0,
			"source":   "list",
			"mainsite": 1,
			"content":  "",
		},
	}
	logs, err := json.Marshal([]any{entry})
	if err != nil {
		return err
	}
	var res struct {
		Code int `json:"code"`
	}
	if err := eapiPost("/api/feedback/weblog", map[string]any{"logs": string(logs)}, &res); err != nil {
		return err
	}
	if res.Code != 200 {
		return fmt.Errorf("netease: scrobble: code %d", res.Code)
	}
	return nil
}
