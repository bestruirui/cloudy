package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// downloadQueueView is the window over the queue: what is downloading,
// what waits, what failed, and which of those the listener chose.
type downloadQueueView struct {
	open     bool
	selected map[int64]bool
	list     ui.ListState
}

type downloadQueueEntry struct {
	track  netease.Track
	active bool
	got    int64
	total  int64
	pos    int    // 1 for the first waiting song
	failed string // why it failed, for a song that did
}

func (a *app) downloadQueueCount() int {
	n := len(a.downloadQueue)
	if a.downloadJob != nil {
		n++
	}
	return n
}

func (a *app) openDownloadQueue() {
	a.queueView.open = true
	a.queueView.selected = map[int64]bool{}
	a.queueView.list = ui.ListState{}
	a.redraw()
}

func (a *app) downloadQueueEntries() []downloadQueueEntry {
	var out []downloadQueueEntry
	if job := a.downloadJob; job != nil {
		out = append(out, downloadQueueEntry{track: job.track, active: true, got: job.got, total: job.total})
	}
	for i, tr := range a.downloadQueue {
		out = append(out, downloadQueueEntry{track: tr, pos: i + 1})
	}
	for _, f := range a.downloadFailed {
		out = append(out, downloadQueueEntry{track: f.Track, failed: f.Reason})
	}
	return out
}

func (a *app) downloadQueueChosen(entries []downloadQueueEntry) int {
	n := 0
	for _, e := range entries {
		if a.queueView.selected[e.track.ID] {
			n++
		}
	}
	return n
}

func (a *app) downloadQueueAll(entries []downloadQueueEntry) bool {
	return len(entries) > 0 && a.downloadQueueChosen(entries) == len(entries)
}

func (a *app) toggleDownloadQueue(id int64) {
	if a.queueView.selected[id] {
		delete(a.queueView.selected, id)
	} else {
		a.queueView.selected[id] = true
	}
	a.redraw()
}

func (a *app) selectAllDownloads(entries []downloadQueueEntry) {
	if a.downloadQueueAll(entries) {
		a.queueView.selected = map[int64]bool{}
	} else {
		a.queueView.selected = make(map[int64]bool, len(entries))
		for _, e := range entries {
			a.queueView.selected[e.track.ID] = true
		}
	}
	a.redraw()
}

// cancelSelectedDownloads takes the chosen songs out of the queue, cancels
// the one downloading if it was chosen, and forgets the chosen ones that
// failed.
func (a *app) cancelSelectedDownloads() {
	selected := a.queueView.selected
	if len(selected) == 0 {
		return
	}
	removed := 0
	active := a.downloadJob != nil && selected[a.downloadJob.track.ID]
	keep := a.downloadQueue[:0]
	for _, tr := range a.downloadQueue {
		if selected[tr.ID] {
			removed++
		} else {
			keep = append(keep, tr)
		}
	}
	a.downloadQueue = keep
	kept := a.downloadFailed[:0]
	for _, f := range a.downloadFailed {
		if selected[f.Track.ID] {
			removed++
		} else {
			kept = append(kept, f)
		}
	}
	a.downloadFailed = kept
	if active {
		removed++
		a.cancelDownload()
	}
	a.queueView.selected = map[int64]bool{}
	if removed > 0 {
		a.toast = fmt.Sprintf("已取消 %d 项下载", removed)
	}
	a.saveDownloadQueue()
	a.redraw()
}

func (a *app) clearDownloadQueue() {
	a.downloadQueue, a.downloadFailed = nil, nil
	a.queueView.selected = map[int64]bool{}
	if a.downloadJob != nil {
		a.cancelDownload()
	}
	a.toast = "已清空下载队列"
	a.saveDownloadQueue()
	a.redraw()
}

// removeDownload takes one song out of the queue, cancels it while it is
// the one downloading, or forgets it once it failed.
func (a *app) removeDownload(id int64) {
	delete(a.queueView.selected, id)
	if a.downloadJob != nil && a.downloadJob.track.ID == id {
		a.cancelDownload()
		return
	}
	for i, tr := range a.downloadQueue {
		if tr.ID == id {
			a.downloadQueue = append(a.downloadQueue[:i], a.downloadQueue[i+1:]...)
			a.toast = "已从下载队列移除：" + tr.Name
			a.saveDownloadQueue()
			a.redraw()
			return
		}
	}
	a.dropFailedDownloads(id)
}

// downloadQueueDialog is the queue's own window.
func (a *app) downloadQueueDialog(c *ui.Context) {
	q := &a.queueView
	if !q.open {
		return
	}
	entries := a.downloadQueueEntries()
	ink := a.inkOn(false)
	w, h := c.Size()
	active, waiting, failed := 0, len(a.downloadQueue), len(a.downloadFailed)
	if a.downloadJob != nil {
		active = 1
	}
	sub := fmt.Sprintf("正在下载 %d 首 · 等待 %d 首", active, waiting)
	if active == 0 {
		sub = fmt.Sprintf("等待下载 %d 首", waiting)
	}
	if failed > 0 {
		sub += fmt.Sprintf(" · 失败 %d 首", failed)
	}
	if len(entries) == 0 {
		sub = "当前没有下载任务"
	}
	a.dialog(c, "download-queue", 0.35, &q.open, func(panel ui.Element) {
		// As tall as what it lists, up to what the window has room for:
		// three songs are not lost in a tall empty pane.
		tall := min(560, h-80)
		panel.Width(min(680, w-80)).Height(min(tall, max(300, 170+float32(len(entries))*66))).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			dialogHeader(c, ink, "下载队列", sub, func() { q.open = false })
			fill, edge := a.cardColors()
			list := ui.Column(c).Grow(1).Shrink(1).MinHeight(0).Margin(0, 28).Radius(20).Background(fill).Border(0.5, edge).Clip()
			list.Children(func() {
				if len(entries) == 0 {
					ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
						ui.Icon(c, iconDownload).Size(26, 26).TextColor(ink.faint)
						ui.Text(c, "下载队列是空的").FontSize(13).TextColor(ink.muted)
					})
					return
				}
				ui.List(c.Key("download-queue-list"), &q.list, len(entries), func(i int) {
					a.downloadQueueRow(c, entries[i], i)
				}).Grow(1).Padding(6)
			})
			chosen := a.downloadQueueChosen(entries)
			dialogFooter(c, func() {
				ui.Textf(c, "共 %d 项，已选 %d 项", len(entries), chosen).FontSize(12).TextColor(ink.muted).Grow(1)
				if failed > 0 && settingButton(c.Key("dq-retry"), ink, "全部重试", iconReload, false, false).Clicked() {
					a.retryDownloads()
				}
				label := "全选"
				if a.downloadQueueAll(entries) {
					label = "取消全选"
				}
				if settingButton(c.Key("dq-all"), ink, label, nil, len(entries) == 0, false).Clicked() {
					a.selectAllDownloads(entries)
				}
				if settingButton(c.Key("dq-cancel"), ink, "取消所选下载", nil, chosen == 0, true).Clicked() {
					a.cancelSelectedDownloads()
				}
				if settingButton(c.Key("dq-clear"), ink, "全部取消", nil, len(entries) == 0, true).Clicked() {
					a.clearDownloadQueue()
				}
			})
		})
	})
}

func (a *app) downloadQueueRow(c *ui.Context, e downloadQueueEntry, i int) {
	pal, ink := a.pal, a.inkOn(false)
	selected := a.queueView.selected[e.track.ID]
	row := ui.CheckboxBase(c.Key(i), &selected).Height(62).PaddingX(12).Gap(12).Radius(14).
		AlignItems(ui.Center).FocusRing(false).Label(e.track.Name)
	bg := ui.Transparent
	if selected {
		bg = pal.accent.Alpha(0.12)
	} else if row.Hovered() {
		bg = pal.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	row.Children(func() {
		a.batchCheck(c, selected)
		a.cover(c, e.track.Album.PicURL, 96, 36, 8).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, e.track.Name).FontSize(13).FontWeight(600).SingleLine()
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, e.track.ArtistNames()).FontSize(11.5).TextColor(ink.muted).SingleLine().Grow(1).Shrink(1).MinWidth(0)
				status, color := fmt.Sprintf("等待中 · 第 %d 位", e.pos), ink.faint
				switch {
				case e.failed != "":
					status, color = "下载失败 · "+e.failed, pal.accent
				case e.active:
					status = "正在下载 · " + sizeText(e.got)
					if e.total > 0 {
						status += " / " + sizeText(e.total)
					}
				}
				ui.Text(c, status).FontSize(10.5).TextColor(color).SingleLine().Shrink(1).MinWidth(0).MaxWidth(260)
			})
			if e.active {
				share := float32(0)
				if e.total > 0 {
					share = min(float32(e.got)/float32(e.total), 1)
				}
				bar := ui.Box(c).Grow(1).MinWidth(40).Height(4).Radius(4).Background(pal.press)
				bar.Draw(func(p *ui.Painter, r ui.Rect) {
					if share > 0 {
						p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * share, H: r.H}, pal.accent, 4)
					}
				})
			}
		})
		if e.failed != "" && iconButton(c.Key("retry"), iconReload, 13, ink.muted, "重新下载").Clicked() {
			a.retryDownloads(e.track.ID)
		}
		label := "移出队列"
		switch {
		case e.active:
			label = "取消下载"
		case e.failed != "":
			label = "不再下载"
		}
		if iconButton(c.Key("remove"), iconX, 13, ink.muted, label).Clicked() {
			a.removeDownload(e.track.ID)
		}
	})
	if row.Changed() {
		a.toggleDownloadQueue(e.track.ID)
	}
}
