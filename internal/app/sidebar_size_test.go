package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Dragging the sidebar's edge widens it, as far as leaves the page its
// room, and keeps the width; a double click makes it as it was.
func TestSidebarDrags(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1180, 780)
	tt.Frame()
	r, ok := tt.Find("调整侧边栏宽度")
	if !ok {
		t.Fatal("no edge to drag")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	for i := 1; i <= 10; i++ {
		tt.Move(x+float32(i*10), y)
		tt.Frame()
	}
	tt.Release(x+100, y)
	tt.Frame()
	if w := a.sideWidth(1180); w < sidebarWidth+90 || w > sidebarWidth+110 {
		t.Errorf("dragged 100 DIPs, the sidebar is %v wide", w)
	}
	if w := a.sideWidth(880); w > 880-pageMin {
		t.Errorf("in the narrowest window it leaves the page %v", 880-w)
	}
	a.settings.SidebarWidth = 1000
	if w := a.sideWidth(1600); w != sidebarMax {
		t.Errorf("dragged far, %v wide", w)
	}
}

// The sidebar at each size, and dragged wide: NM_FRAMES saves them.
func TestSidebarSizeFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, c := range []struct {
		name, size string
		width      int
	}{{"standard", "", 0}, {"huge-wide", "huge", 330}} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.settings.SidebarSize, a.settings.SidebarWidth = c.size, c.width
		a.profile = &netease.Profile{UserID: 1, Nickname: "nya_owo"}
		a.mine = []netease.Playlist{{ID: 1, Name: "nya_owo喜欢的音乐", SpecialType: 5}, {ID: 2, Name: "此生无悔入MC（建造专用BGM合集）"}}
		a.saved = []netease.Playlist{{ID: 3, Name: "大叔追云彩视频BGM收藏"}, {ID: 4, Name: "网易云欧美热歌榜"}}
		tt := ui.NewTester(a.view, 1180, 780)
		tt.SetDark(true)
		tt.Frame()
		tt.Advance(time.Second)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, "sidebar-"+c.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}
