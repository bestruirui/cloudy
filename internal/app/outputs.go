package app

import (
	"time"

	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
)

// outputList is the system's audio outputs as last listed, for the
// settings: listed again when they show after a while, as devices come
// and go.
type outputList struct {
	outs []audio.Output
	at   time.Time
}

// outputs lists the outputs, at most once every few seconds.
func (a *app) outputs() []audio.Output {
	if time.Since(a.outputList.at) > 3*time.Second {
		a.outputList.outs, _ = audio.Outputs()
		a.outputList.at = time.Now()
	}
	return a.outputList.outs
}

// applyOutput plays through the output the listener chose.
func (a *app) applyOutput() { audio.SetOutput(a.settings.OutputDevice) }

// outputRow lets the listener choose the output music plays through, where
// the system lists them.
func (a *app) outputRow(c *ui.Context, row func(string, string, func())) {
	outs := a.outputs()
	if len(outs) == 0 {
		return
	}
	const follow = "跟随系统默认"
	labels := []string{follow}
	chosen := follow
	desc := "耳机和音箱来回切换时，可以固定从其中一个播放"
	found := a.settings.OutputDevice == ""
	for _, o := range outs {
		label := o.Name
		if o.Default {
			label += "（默认）"
		}
		labels = append(labels, label)
		if o.ID == a.settings.OutputDevice {
			chosen, found = label, true
		}
	}
	if !found {
		desc = "所选设备现在不可用，暂时从系统默认设备播放"
	}
	row("输出设备", desc, func() {
		if a.paneSelect(c.Key("output-device"), "输出设备", &chosen, labels) {
			a.settings.OutputDevice = ""
			for i, o := range outs {
				if labels[i+1] == chosen {
					a.settings.OutputDevice = o.ID
				}
			}
			a.applyOutput()
			a.changed()
		}
	})
}
