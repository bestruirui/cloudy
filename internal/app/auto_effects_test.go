package app

import (
	"testing"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// A genre as NetEase says it picks its preset, the first known of several;
// one it does not know picks none.
func TestPresetForGenres(t *testing.T) {
	for _, c := range []struct {
		genres []string
		want   string
	}{
		{[]string{"流行-华语流行"}, "pop"},
		{[]string{"摇滚-另类摇滚"}, "rock"},
		{[]string{"说唱-中文说唱"}, "bass"},
		{[]string{"未知", "电子-浩室"}, "electronic"},
		{[]string{"古典-交响乐"}, "classical"},
	} {
		p, _, ok := presetForGenres(c.genres)
		if !ok || p.id != c.want {
			t.Errorf("%q: %q %v, want %q", c.genres, p.id, ok, c.want)
		}
	}
	if _, _, ok := presetForGenres([]string{"未知"}); ok {
		t.Error("an unknown genre picked a preset")
	}
}

// A mood comes from NetEase's tags first, else from what the lyrics say
// over and over; lyrics that say little of any pick none.
func TestMoodOf(t *testing.T) {
	if m, _ := moodOf([]string{"思念", "浪漫", "治愈"}, nil); m == nil || m.label != "伤感" {
		t.Errorf("思念 took %v", m)
	}
	lines := []lyrics.Line{{Text: "燃烧吧我的梦想"}, {Text: "冲向远方 无畏"}, {Text: "勇敢地飞翔"}}
	if m, by := moodOf(nil, lines); m == nil || m.label != "热血" || by == "" {
		t.Errorf("rousing lyrics took %v %q", m, by)
	}
	if m, _ := moodOf(nil, []lyrics.Line{{Text: "今天天气不错"}}); m != nil {
		t.Errorf("plain lyrics took %v", m.label)
	}
}

// Chosen for a song, the effects are its genre's preset with the
// listener's own placing round the head, and the pane says what chose them.
func TestAutoChooses(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Effects.On, a.settings.Effects.Spatial = true, "360"
	a.settings.EffectsPreset = smartPreset
	tr := netease.Track{ID: 5, Name: "song"}
	a.pl.Cue([]netease.Track{tr})
	a.autoChoose([]string{"摇滚-硬摇滚"}, nil, &tr)
	e := a.currentEffects()
	if e.Bands != presetByID("rock").bands || e.Spatial != "360" || !e.On {
		t.Errorf("chose %+v", e)
	}
	if a.auto.note != "已识别曲风：摇滚-硬摇滚" {
		t.Errorf("note %q", a.auto.note)
	}
}
