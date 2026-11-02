package app

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// globalKeys are the keys that work from any app: the first of each that no
// other app holds, for no one key is free on every computer. Ctrl+Alt+Space,
// for one, is how Claude's desktop app is called up.
var globalKeys = []struct {
	keys []string
	do   func(a *app)
}{
	{[]string{"Ctrl+Alt+P", "Ctrl+Alt+Space", "Ctrl+Alt+Home"}, func(a *app) { a.pl.TogglePlay() }},
	{[]string{"Ctrl+Alt+Right"}, func(a *app) { a.pl.Next() }},
	{[]string{"Ctrl+Alt+Left"}, func(a *app) { a.pl.Prev() }},
	{[]string{"Ctrl+Alt+Up"}, func(a *app) { a.nudgeVolume(0.05) }},
	{[]string{"Ctrl+Alt+Down"}, func(a *app) { a.nudgeVolume(-0.05) }},
}

// applyGlobalKeys registers the keys that work from any app, or lets them go.
// Other apps may hold all of those of something, which keysTaken counts.
func (a *app) applyGlobalKeys() {
	a.keysTaken, a.playKey = 0, ""
	for i, k := range globalKeys {
		for _, keys := range k.keys {
			mygo.GlobalShortcut.Unregister(keys)
		}
		if !a.settings.GlobalKeys {
			continue
		}
		do, taken := k.do, true
		for _, keys := range k.keys {
			err := mygo.GlobalShortcut.Register(keys, func() {
				do(a)
				a.redraw()
			})
			if err == nil {
				taken = false
				if i == 0 {
					a.playKey = keys
				}
				break
			}
			log.Printf("shortcut %s: %v", keys, err)
		}
		if taken {
			a.keysTaken++
		}
	}
}

func (a *app) nudgeVolume(by float64) {
	a.volume = math.Max(0, math.Min(a.volume+by, 1))
	a.pl.SetVolume(a.volume)
}

// sleepChoices are how long music can play on for.
var sleepChoices = []struct {
	label   string
	minutes int
}{
	{"不定时", 0},
	{"15 分钟后", 15},
	{"30 分钟后", 30},
	{"45 分钟后", 45},
	{"60 分钟后", 60},
	{"90 分钟后", 90},
}

// setSleep pauses the music after so many minutes, none to not.
func (a *app) setSleep(minutes int) {
	if a.sleepTimer != nil {
		a.sleepTimer.Stop()
		a.sleepTimer = nil
	}
	a.sleepMinutes, a.sleepAt = minutes, time.Time{}
	if minutes == 0 {
		return
	}
	d := time.Duration(minutes) * time.Minute
	a.sleepAt = time.Now().Add(d)
	a.sleepTimer = time.AfterFunc(d, func() { a.post(a.sleepNow) })
}

// sleepNow is the time up: the music fades out as it pauses.
func (a *app) sleepNow() {
	a.setSleep(0)
	if a.pl.Playing() {
		a.pl.TogglePlay()
		a.toast = "定时结束，已暂停播放"
	}
}

// sleepRow lets the listener choose when the music stops, and says when.
func (a *app) sleepRow(c *ui.Context, row func(string, string, func())) {
	desc := "到时间后暂停播放，声音会慢慢淡出；重新打开软件后需要再设置"
	if !a.sleepAt.IsZero() {
		left := time.Until(a.sleepAt)
		desc = "还有 " + longTime(left.Round(time.Minute)) + "后暂停播放"
		// Asks for the settings to say so again.
		c.After(30 * time.Second)
	}
	row("定时关闭", desc, func() {
		labels := make([]string, len(sleepChoices))
		chosen := sleepChoices[0].label
		for i, s := range sleepChoices {
			labels[i] = s.label
			if s.minutes == a.sleepMinutes {
				chosen = s.label
			}
		}
		if a.paneSelect(c.Key("sleep"), "定时关闭", &chosen, labels) {
			for _, s := range sleepChoices {
				if s.label == chosen {
					a.setSleep(s.minutes)
				}
			}
		}
	})
}

// lyricShift is how far the lyrics of a song are moved on from what plays,
// as the listener set them.
func (a *app) lyricShift(id int64) time.Duration {
	return time.Duration(a.settings.LyricShift[id]) * time.Millisecond
}

// lyricPos is where in the lyrics of the song playing the music is.
func (a *app) lyricPos() time.Duration {
	pos := a.pl.Position()
	if tr := a.pl.Current(); tr != nil {
		pos += a.lyricShift(tr.ID)
	}
	return pos
}

// lyricScale is how much larger than as designed the lyrics are.
func (a *app) lyricScale() float32 {
	if s := a.settings.LyricScale; s > 0 {
		return float32(s)
	}
	return 1
}

// lyricTools are at the foot of the lyrics while the pointer is over the
// window: their size, and how early or late they run, kept for each song.
func (a *app) lyricTools(c *ui.Context, id int64) {
	s := &a.settings
	shift := s.LyricShift[id]
	move := func(by int) {
		if s.LyricShift == nil {
			s.LyricShift = map[int64]int{}
		}
		if shift += by; shift == 0 {
			delete(s.LyricShift, id)
		} else {
			s.LyricShift[id] = shift
		}
		a.changed()
	}
	size := func(by float64) {
		s.LyricScale = math.Max(0.7, math.Min(float64(a.lyricScale())+by, 1.6))
		a.changed()
	}
	bar := ui.Row(c.Key("lyric-tools")).Absolute().Right(10).Bottom(14).Gap(2).Padding(4).AlignItems(ui.Center).Label("歌词调整")
	a.pane(bar, 20, true)
	reveal(bar, a.npPointer).Children(func() {
		chip := func(key, label, says string, do func()) ui.Element {
			b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(9).Radius(14).Center().FocusRing(false).Label(says)
			if b.Hovered() {
				b.Background(white.Alpha(0.12))
			}
			b.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(white.Alpha(0.85)).SingleLine() })
			if b.Clicked() {
				do()
			}
			return b
		}
		chip("smaller", "A−", "歌词字号减小", func() { size(-0.1) })
		chip("larger", "A+", "歌词字号增大", func() { size(0.1) })
		ui.Box(c).Size(1, 14).Margin(0, 3).Background(white.Alpha(0.14))
		chip("earlier", "提前", "歌词提前 0.1 秒", func() { move(100) })
		// Always there, and as wide: a button that came and went would move
		// the ones beside it from under the pointer, between two clicks.
		chip("reset", fmt.Sprintf("%+.1fs", float64(shift)/1000), "恢复歌词时间", func() { move(-shift) }).Width(52)
		chip("later", "延后", "歌词延后 0.1 秒", func() { move(-100) })
	})
}
