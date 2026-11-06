package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// downloadsPage lists the songs saved to disk, and what is on its way.
func (a *app) downloadsPage(c *ui.Context) {
	if !a.downloadsLoaded {
		a.loadDownloads(false)
	}
	tracks := a.downloadTracks()
	if a.downloadsErr != "" && len(tracks) == 0 && a.downloadJob == nil {
		a.centered(c, a.downloadsErr, func() { a.loadDownloads(true) })
		return
	}
	if len(tracks) == 0 && a.downloadJob == nil && len(a.downloadQueue) == 0 {
		a.downloadsEmpty(c)
		return
	}
	a.toolRows(c, tracks, true, a.downloadsHeader)
}

// downloadArtURL is the cover of the newest song saved, or of the one
// arriving while there is no other.
func (a *app) downloadArtURL() string {
	if len(a.downloads) > 0 {
		return a.downloads[0].Track.Album.PicURL
	}
	if a.downloadJob != nil {
		return a.downloadJob.track.Album.PicURL
	}
	return ""
}

// downloadCovers are the covers the page can stack beside its title.
func (a *app) downloadCovers() []string {
	urls := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, it := range a.downloads {
		url := it.Track.Album.PicURL
		if url != "" && !seen[url] {
			seen[url] = true
			urls = append(urls, url)
			if len(urls) == 3 {
				break
			}
		}
	}
	return urls
}

// downloadsHeader is the page as a playlist is: its newest cover, what it
// says, where it is kept, and what plays or reads it.
func (a *app) downloadsHeader(c *ui.Context) {
	p := a.pal
	tracks := a.downloadTracks()
	var total time.Duration
	for i := range tracks {
		total += tracks[i].Length()
	}
	meta := "保存在本机"
	if len(a.downloads) > 0 {
		meta = fmt.Sprintf("保存在本机 · %d 首 · %s · %s", len(a.downloads), longTime(total), sizeText(a.downloadBytes()))
	}
	head := ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0)
	head.Children(func() {
		a.headCover(c, a.downloadArtURL())
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
			ui.Text(c, "本地下载").FontSize(12).FontWeight(600).TextColor(p.accent)
			ui.Text(c, "下载歌曲").FontSize(30).FontWeight(800).LetterSpacing(-0.6).MaxLines(1)
			ui.Text(c, meta).FontSize(13).TextColor(p.muted)
			if a.downloadBatchActive() {
				ui.Text(c, "选择歌曲，完成后在下方删除").FontSize(12.5).TextColor(p.muted).Margin(10, 0, 0, 0)
			} else {
				ui.Row(c).Wrap().GapX(10).GapY(8).Margin(10, 0, 0, 0).Children(func() {
					if len(a.downloads) > 0 {
						if pillButton(c.Key("play"), iconPlay, "播放", p.accent, p.onAccent).Clicked() {
							a.playList(a.downloadTracks(), false)
						}
						if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
							a.playList(a.downloadTracks(), true)
						}
					}
					if pillButton(c.Key("open"), iconFolder, "打开文件夹", p.hover.Over(p.bg), p.text).Clicked() {
						a.openDownloadFolder()
					}
					if pillButton(c.Key("change"), iconFolder, "更换文件夹", p.hover.Over(p.bg), p.text).Clicked() {
						a.pickDownloadFolder()
					}
					if pillButton(c.Key("rescan"), iconReload, "重新扫描", p.hover.Over(p.bg), p.text).Clicked() {
						a.loadDownloads(true)
						a.toast = "正在重新读取下载歌曲…"
					}
					count := a.downloadQueueCount()
					queueLabel := "下载队列"
					if count > 0 {
						queueLabel = fmt.Sprintf("下载队列 %d", count)
					}
					if pillButton(c.Key("queue"), iconQueue, queueLabel, p.hover.Over(p.bg), p.text).Clicked() {
						a.openDownloadQueue()
					}
					if len(a.downloads) > 0 {
						if pillButton(c.Key("batch-delete"), iconTrash, "批量删除", p.hover.Over(p.bg), p.text).Clicked() {
							a.startDownloadBatch()
						}
					}
				})
			}
			a.downloadProgress(c)
		})
		a.coverStackURLs(c, a.downloadCovers())
	})
}

// downloadProgress shows the song arriving, how far it has come, and what
// waits behind it.
func (a *app) downloadProgress(c *ui.Context) {
	p := a.pal
	job := a.downloadJob
	if job == nil {
		return
	}
	share := float32(0)
	if job.total > 0 {
		share = min(float32(job.got)/float32(job.total), 1)
	}
	ui.Column(c).Gap(5).Margin(6, 0, 0, 0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconDownload).Size(13, 13).TextColor(p.accent)
			ui.Text(c, "正在下载 · "+job.track.Name).FontSize(12).FontWeight(600).TextColor(p.text).SingleLine().Grow(1).Shrink(1).MinWidth(0)
			if pillButton(c.Key("download-cancel"), nil, "取消", p.hover.Over(p.bg), p.text).Clicked() {
				a.cancelDownload()
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			bar := ui.Box(c).Grow(1).MinWidth(40).Height(5).Radius(3).Background(p.press)
			bar.Draw(func(pt *ui.Painter, r ui.Rect) {
				if share > 0 {
					pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * share, H: r.H}, p.accent, 3)
				}
			})
			if job.total > 0 {
				ui.Text(c, fmt.Sprintf("%s / %s", sizeText(job.got), sizeText(job.total))).FontSize(10.5).FontFeatures("tnum").TextColor(p.muted)
			} else {
				ui.Text(c, sizeText(job.got)).FontSize(10.5).FontFeatures("tnum").TextColor(p.muted)
			}
			if len(a.downloadQueue) > 0 {
				ui.Text(c, fmt.Sprintf("还有 %d 首", len(a.downloadQueue))).FontSize(10.5).TextColor(p.faint)
			}
		})
	})
}

// downloadsEmpty is the page with nothing saved: where downloads go, and
// how to start one.
func (a *app) downloadsEmpty(c *ui.Context) {
	p := a.pal
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(14).Children(func() {
		ui.Icon(c, iconDownload).Size(56, 56).TextColor(p.faint)
		ui.Text(c, "还没有下载歌曲").FontSize(15).FontWeight(600)
		ui.Text(c, "在任意歌曲的右键菜单里选择「下载到本机」，也可以点歌曲旁的「更多」。下载会按设置里的音质保存到本机。").FontSize(12.5).TextColor(p.muted).MaxWidth(560).TextAlign(ui.Center)
		ui.Row(c).Wrap().GapX(10).GapY(8).Children(func() {
			if pillButton(c, iconFolder, "打开文件夹", p.hover.Over(p.bg), p.text).Clicked() {
				a.openDownloadFolder()
			}
			if pillButton(c, iconFolder, "更换文件夹", p.hover.Over(p.bg), p.text).Clicked() {
				a.pickDownloadFolder()
			}
			if pillButton(c, iconReload, "重新扫描", p.hover.Over(p.bg), p.text).Clicked() {
				a.loadDownloads(true)
			}
			if pillButton(c, iconHome, "去为你推荐", p.hover.Over(p.bg), p.text).Clicked() {
				a.open(route{page: pageHome})
			}
		})
	})
}

// sizeText says bytes in the unit that keeps them short.
func sizeText(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
