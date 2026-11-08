package app

import (
	"time"

	"github.com/egoist/mygo"
)

// progressSteps is how finely the taskbar button shows the song's progress:
// finer than its few dozen pixels, and coarse enough that a second does not
// always move it.
const progressSteps = 1000

// A progressShown is what the taskbar button shows of the song: how far it
// has played, in steps, and whether it is paused.
type progressShown struct {
	on     bool
	paused bool
	step   int
}

// syncProgress shows on the taskbar button how far the song has played, if
// the listener wants it to: yellow while it is paused. It tells the taskbar
// only of a change.
func (a *app) syncProgress() {
	if a.win == nil || a.pl == nil {
		return
	}
	var want progressShown
	if d := a.pl.Duration(); a.settings.TaskbarProgress && a.hasSong() && d > 0 {
		at := min(max(float64(a.pl.Position())/float64(d), 0), 1)
		want = progressShown{on: true, paused: !a.pl.Playing() && !a.pl.Loading(), step: int(at * progressSteps)}
	}
	if want == a.progressAt {
		return
	}
	a.progressAt = want
	switch {
	case !want.on:
		a.win.SetProgressBar(mygo.ProgressBar{})
	case want.paused:
		a.win.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressPaused, Value: float64(want.step) / progressSteps})
	default:
		a.win.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressNormal, Value: float64(want.step) / progressSteps})
	}
}

// followProgress moves the taskbar's progress on every second, whether the
// window draws or not: minimized or in the tray, it doesn't.
func (a *app) followProgress() {
	go func() {
		for range time.Tick(time.Second) {
			mygo.RunOnMain(func() {
				if a.settings.TaskbarProgress || a.progressAt.on {
					a.syncProgress()
				}
			})
		}
	}()
}
