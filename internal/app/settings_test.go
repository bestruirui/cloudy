package app

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Switching categories keeps edited settings and each category's scroll position.
func TestSettingsCategoryNavigation(t *testing.T) {
	checks, downloads, checked := true, false, 0
	defer fakeUpdater(&checks, &downloads, &checked)()
	a := newTestApp()
	a.settings = defaultSettings()
	a.settingsOpen, a.updates = true, true
	tt := ui.NewTester(a.view, 1000, 700)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	choose := func(label string, want settingsCategory) {
		t.Helper()
		if err := tt.Click("设置分类：" + label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.settingsTab != want {
			t.Fatalf("%s opened category %d", label, a.settingsTab)
		}
	}
	toggle := func(label, desc string) {
		t.Helper()
		title, ok1 := tt.Find(label)
		note, ok2 := tt.Find(desc)
		pane, ok3 := tt.Find(settingsCategories[a.settingsTab].label + "设置项")
		if !ok1 || !ok2 || !ok3 {
			t.Fatalf("the %s control is not in this category", label)
		}
		tt.ClickAt(pane.X+pane.W-24-16-12, (title.Y+note.Y+note.H)/2)
		tt.Frame()
	}
	choose("外观", settingsAppearance)
	wasAccent := a.settings.AccentCover
	a.settings.AccentCover = !wasAccent
	pane, _ := tt.Find("外观设置项")
	tt.Scroll(pane.X+40, pane.Y+pane.H/2, 0, 320)
	tt.Advance(time.Second)
	tt.Frame()
	at := a.settingsScroll[settingsAppearance].Y
	if at <= 0 {
		t.Fatal("the appearance category did not scroll")
	}
	choose("常规", settingsGeneral)
	if a.settingsScroll[settingsGeneral].Y != 0 || a.settings.AccentCover == wasAccent {
		t.Fatal("switching categories lost the edit or shared the scroll position")
	}
	choose("外观", settingsAppearance)
	if math.Abs(float64(a.settingsScroll[settingsAppearance].Y-at)) > 1 {
		t.Fatal("returning to appearance lost its scroll position")
	}
	choose("播放", settingsPlayback)
	choose("关于", settingsAbout)
	toggle("自动安装更新", "在后台下载并装好新版本，不弹窗，下次打开软件时生效")
	if !downloads {
		t.Fatal("the update preference did not change")
	}
	choose("常规", settingsGeneral)
	choose("关于", settingsAbout)
	if !downloads {
		t.Fatal("returning to about lost the update preference")
	}
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatal("the about category did not check for updates")
	}
}

// With NM_FRAMES set, saves each settings category at regular and compact sizes.
func TestSettingsCategoryFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	checks, downloads, checked := true, false, 0
	defer fakeUpdater(&checks, &downloads, &checked)()
	for _, size := range []struct{ w, h int }{{1180, 780}, {880, 580}} {
		for category := settingsGeneral; category < settingsCategoryCount; category++ {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settingsOpen, a.updates = true, true
			a.settingsTab = category
			a.profile = &netease.Profile{UserID: 1, Nickname: "试听者"}
			tt := ui.NewTester(a.view, size.w, size.h)
			tt.SetDark(true)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			name := fmt.Sprintf("settings-%d-%d.png", int(size.w), category)
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
