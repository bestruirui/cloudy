package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// uploadQueueView is the window over the uploads, as the download queue's:
// what goes up, what waits, what failed, and which of those are chosen.
type uploadQueueView struct {
	open     bool
	selected map[string]bool // by uploadKey
	list     ui.ListState
}

type uploadQueueEntry struct {
	item   cloudUpload
	active bool
	sent   int64
	all    int64
	pos    int // 1 for the first waiting song
}

func (e uploadQueueEntry) key() string { return uploadKey(e.item.Path) }

func (a *app) uploadQueueCount() int {
	n := len(a.uploadQueue) + len(a.uploadFailed)
	if a.uploadJob != nil {
		n++
	}
	return n
}

func (a *app) openUploadQueue() {
	a.uploadView = uploadQueueView{open: true, selected: map[string]bool{}}
	a.redraw()
}

func (a *app) uploadQueueEntries() []uploadQueueEntry {
	var out []uploadQueueEntry
	if job := a.uploadJob; job != nil {
		out = append(out, uploadQueueEntry{item: job.item, active: true, sent: job.sent, all: job.all})
	}
	for i, u := range a.uploadQueue {
		out = append(out, uploadQueueEntry{item: u, pos: i + 1})
	}
	for _, f := range a.uploadFailed {
		out = append(out, uploadQueueEntry{item: f})
	}
	return out
}

func (a *app) uploadQueueChosen(entries []uploadQueueEntry) int {
	n := 0
	for _, e := range entries {
		if a.uploadView.selected[e.key()] {
			n++
		}
	}
	return n
}

func (a *app) uploadQueueAll(entries []uploadQueueEntry) bool {
	return len(entries) > 0 && a.uploadQueueChosen(entries) == len(entries)
}

func (a *app) selectAllUploads(entries []uploadQueueEntry) {
	all := a.uploadQueueAll(entries)
	a.uploadView.selected = map[string]bool{}
	if !all {
		for _, e := range entries {
			a.uploadView.selected[e.key()] = true
		}
	}
	a.redraw()
}

// cancelSelectedUploads takes the chosen songs out of the queue, cancels the
// one going up if it was chosen, and forgets the chosen ones that failed.
func (a *app) cancelSelectedUploads() {
	selected := a.uploadView.selected
	if len(selected) == 0 {
		return
	}
	removed := 0
	active := a.uploadJob != nil && selected[uploadKey(a.uploadJob.item.Path)]
	keep := a.uploadQueue[:0]
	for _, u := range a.uploadQueue {
		if selected[uploadKey(u.Path)] {
			removed++
		} else {
			keep = append(keep, u)
		}
	}
	a.uploadQueue = keep
	kept := a.uploadFailed[:0]
	for _, f := range a.uploadFailed {
		if selected[uploadKey(f.Path)] {
			removed++
		} else {
			kept = append(kept, f)
		}
	}
	a.uploadFailed = kept
	if active {
		removed++
		a.cancelUpload()
	}
	a.uploadView.selected = map[string]bool{}
	if removed > 0 {
		a.toast = fmt.Sprintf("已取消 %d 项上传", removed)
	}
	a.saveUploadQueue()
	a.redraw()
}

func (a *app) clearUploadQueue() {
	a.uploadQueue, a.uploadFailed = nil, nil
	a.uploadView.selected = map[string]bool{}
	if a.uploadJob != nil {
		a.cancelUpload()
	}
	a.toast = "已清空上传列表"
	a.saveUploadQueue()
	a.redraw()
}

// removeUpload takes one song out of the queue, cancels it while it goes
// up, or forgets it once it failed.
func (a *app) removeUpload(path string) {
	key := uploadKey(path)
	delete(a.uploadView.selected, key)
	if a.uploadJob != nil && uploadKey(a.uploadJob.item.Path) == key {
		a.cancelUpload()
		return
	}
	for i, u := range a.uploadQueue {
		if uploadKey(u.Path) == key {
			a.uploadQueue = append(a.uploadQueue[:i], a.uploadQueue[i+1:]...)
			a.toast = "已从上传列表移除：" + u.name()
			a.saveUploadQueue()
			a.redraw()
			return
		}
	}
	a.dropFailedUploads(path)
	a.saveUploadQueue()
	a.redraw()
}

// uploadQueueDialog is the upload list's own window.
func (a *app) uploadQueueDialog(c *ui.Context) {
	q := &a.uploadView
	if !q.open {
		return
	}
	entries := a.uploadQueueEntries()
	ink := a.inkOn(false)
	w, h := c.Size()
	waiting, failed := len(a.uploadQueue), len(a.uploadFailed)
	sub := fmt.Sprintf("等待上传 %d 首", waiting)
	if a.uploadJob != nil {
		sub = fmt.Sprintf("正在上传 1 首 · 等待 %d 首", waiting)
	}
	if failed > 0 {
		sub += fmt.Sprintf(" · 失败 %d 首", failed)
	}
	if len(entries) == 0 {
		sub = "当前没有上传任务"
	}
	a.dialog(c, "upload-queue", 0.35, &q.open, func(panel ui.Element) {
		tall := min(560, h-80)
		panel.Width(min(680, w-80)).Height(min(tall, max(300, 170+float32(len(entries))*66))).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			dialogHeader(c, ink, "上传列表", sub, func() { q.open = false })
			fill, edge := a.cardColors()
			list := ui.Column(c).Grow(1).Shrink(1).MinHeight(0).Margin(0, 28).Radius(20).Background(fill).Border(0.5, edge).Clip()
			list.Children(func() {
				if len(entries) == 0 {
					ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
						ui.Icon(c, iconUpload).Size(26, 26).TextColor(ink.faint)
						ui.Text(c, "上传列表是空的").FontSize(13).TextColor(ink.muted)
					})
					return
				}
				ui.List(c.Key("upload-queue-list"), &q.list, len(entries), func(i int) {
					a.uploadQueueRow(c, entries[i], i)
				}).Grow(1).Padding(6)
			})
			chosen := a.uploadQueueChosen(entries)
			dialogFooter(c, func() {
				ui.Textf(c, "共 %d 项，已选 %d 项", len(entries), chosen).FontSize(12).TextColor(ink.muted).Grow(1)
				if settingButton(c.Key("uq-add"), ink, "添加音乐", iconUpload, false, false).Clicked() {
					a.pickUploads()
				}
				if failed > 0 && settingButton(c.Key("uq-retry"), ink, "全部重试", iconReload, false, false).Clicked() {
					a.retryUploads()
				}
				label := "全选"
				if a.uploadQueueAll(entries) {
					label = "取消全选"
				}
				if settingButton(c.Key("uq-all"), ink, label, nil, len(entries) == 0, false).Clicked() {
					a.selectAllUploads(entries)
				}
				if settingButton(c.Key("uq-cancel"), ink, "取消所选", nil, chosen == 0, true).Clicked() {
					a.cancelSelectedUploads()
				}
				if settingButton(c.Key("uq-clear"), ink, "全部取消", nil, len(entries) == 0, true).Clicked() {
					a.clearUploadQueue()
				}
			})
		})
	})
}

func (a *app) uploadQueueRow(c *ui.Context, e uploadQueueEntry, i int) {
	pal, ink := a.pal, a.inkOn(false)
	key := e.key()
	selected := a.uploadView.selected[key]
	row := ui.CheckboxBase(c.Key(i), &selected).Height(62).PaddingX(12).Gap(12).Radius(14).
		AlignItems(ui.Center).FocusRing(false).Label(e.item.name())
	bg := ui.Transparent
	if selected {
		bg = pal.accent.Alpha(0.12)
	} else if row.Hovered() {
		bg = pal.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	row.Children(func() {
		a.batchCheck(c, selected)
		ui.Row(c).Size(36, 36).Shrink(0).Radius(8).Background(pal.press).Center().Children(func() {
			ui.Icon(c, iconNote).Size(16, 16).TextColor(ink.faint)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, e.item.name()).FontSize(13).FontWeight(600).SingleLine()
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				artist := e.item.Artist
				if artist == "" {
					artist = sizeText(e.item.Size)
				}
				ui.Text(c, artist).FontSize(11.5).TextColor(ink.muted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
				status, color := fmt.Sprintf("等待中 · 第 %d 位", e.pos), ink.faint
				switch {
				case e.item.Reason != "":
					status, color = "上传失败 · "+e.item.Reason, pal.accent
				case e.active && e.sent == 0:
					status = "正在准备…"
				case e.active:
					status = "正在上传 · " + sizeText(e.sent)
					if e.all > 0 {
						status += " / " + sizeText(e.all)
					}
				}
				ui.Text(c, status).FontSize(10.5).TextColor(color).SingleLine().Shrink(1).MinWidth(0).MaxWidth(260)
			})
			if e.active {
				share := float32(0)
				if e.all > 0 {
					share = min(float32(e.sent)/float32(e.all), 1)
				}
				bar := ui.Box(c).Grow(1).MinWidth(40).Height(4).Radius(4).Background(pal.press)
				bar.Draw(func(p *ui.Painter, r ui.Rect) {
					if share > 0 {
						p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * share, H: r.H}, pal.accent, 4)
					}
				})
			}
		})
		if e.item.Reason != "" && iconButton(c.Key("retry"), iconReload, 13, ink.muted, "重新上传").Clicked() {
			a.retryUploads(e.item.Path)
		}
		label := "移出列表"
		switch {
		case e.active:
			label = "取消上传"
		case e.item.Reason != "":
			label = "不再上传"
		}
		if iconButton(c.Key("remove"), iconX, 13, ink.muted, label).Clicked() {
			a.removeUpload(e.item.Path)
		}
	})
	if row.Changed() {
		if a.uploadView.selected[key] {
			delete(a.uploadView.selected, key)
		} else {
			a.uploadView.selected[key] = true
		}
		a.redraw()
	}
}
