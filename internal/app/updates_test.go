package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// fakeUpdater swaps the update plugin's preferences for ones in memory,
// counting the checks asked for, until the returned function puts them
// back.
func fakeUpdater(checks, downloads *bool, checked *int) func() {
	check, getChecks, setChecks := checkForUpdates, updateChecks, setUpdateChecks
	getDownloads, setDownloads, last := updateDownloads, setUpdateDownloads, lastUpdateCheck
	checkForUpdates = func() { *checked++ }
	updateChecks = func() bool { return *checks }
	setUpdateChecks = func(on bool) { *checks = on }
	updateDownloads = func() bool { return *downloads }
	setUpdateDownloads = func(on bool) { *downloads = on }
	lastUpdateCheck = func() time.Time { return time.Time{} }
	return func() {
		checkForUpdates, updateChecks, setUpdateChecks = check, getChecks, setChecks
		updateDownloads, setUpdateDownloads, lastUpdateCheck = getDownloads, setDownloads, last
	}
}

// 自动安装更新 has the updater install updates without asking, and turning
// off the reminder turns checking off in it and takes the choice away too,
// as nothing would find the updates to install.
func TestUpdateSettings(t *testing.T) {
	checks, downloads, checked := true, false, 0
	defer fakeUpdater(&checks, &downloads, &checked)()
	a := newTestApp()
	a.settings = defaultSettings()
	a.updates = true
	tt := ui.NewTester(a.settingsBody, 560, 3000)
	tt.Frame()
	// The row's title is labelled as its switch is: click the switch, at
	// the row's end.
	toggle := func(title, desc string) {
		t.Helper()
		top, ok1 := tt.Find(title)
		bottom, ok2 := tt.Find(desc)
		if !ok1 || !ok2 {
			t.Fatalf("no row %s", title)
		}
		tt.ClickAt(560-24-16-12, (top.Y+bottom.Y+bottom.H)/2)
		tt.Frame()
	}
	toggle("自动安装更新", "在后台下载并装好新版本，不弹窗，下次打开软件时生效")
	if !downloads {
		t.Fatal("still asks before installing updates")
	}
	toggle("提醒新版本", "每天检查一次是否有新版本")
	if checks {
		t.Fatal("checks for updates still")
	}
	if tt.HasText("自动安装更新") {
		t.Error("offers installing updates with no checks to find them")
	}
}

// 检查更新 in the settings checks for updates as the user asked.
func TestCheckForUpdatesFromSettings(t *testing.T) {
	checks, downloads, checked := true, false, 0
	defer fakeUpdater(&checks, &downloads, &checked)()
	a := newTestApp()
	a.settings = defaultSettings()
	a.updates = true
	a.profile = &netease.Profile{UserID: 1}
	tt := ui.NewTester(a.settingsBody, 560, 3000)
	tt.Frame()
	if err := tt.Click("检查更新"); err != nil {
		t.Fatal(err)
	}
	if checked != 1 {
		t.Fatalf("checked %d times", checked)
	}
}

// Cloud-nya's data moves to Cloudy's directory, rather than the data of the
// name before it, kept beside it too.
func TestMoveFromCloudNya(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Cloudy")
	for name, text := range map[string]string{lastName: "nya", oldName: "older"} {
		os.MkdirAll(filepath.Join(root, name), 0o700)
		os.WriteFile(filepath.Join(root, name, "settings.json"), []byte(text), 0o600)
	}
	os.MkdirAll(dir, 0o700)
	moveFromOldNames(dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "nya" {
		t.Fatalf("settings %q after moving", b)
	}
	if _, err := os.Stat(filepath.Join(root, oldName, "settings.json")); err != nil {
		t.Fatalf("the older data went too: %v", err)
	}
}

// The data kept under the app's old name moves to the directory of its
// name, which Path made empty, but not over data kept there already.
func TestMoveOldData(t *testing.T) {
	root := t.TempDir()
	old, dir := filepath.Join(root, oldName), filepath.Join(root, "MyGo 云音乐")
	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "settings.json"), []byte("old"), 0o600)
	os.MkdirAll(dir, 0o700)
	moveDir(old, dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "old" {
		t.Fatalf("settings %q after moving", b)
	}

	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "settings.json"), []byte("older"), 0o600)
	moveDir(old, dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "old" {
		t.Fatalf("settings %q, moved over what was there", b)
	}
}
