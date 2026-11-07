package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// updateDialog shows the update in the app's own glass: the check under way,
// the release notes of an update, the download of it and the restart into it.
func (a *app) updateDialog(c *ui.Context) {
	u := &a.update
	if !u.open {
		return
	}
	p, ink := a.pal, a.inkOn(false)
	a.dialog(c, "update", 0.35, &u.open, func(panel ui.Element) {
		a.pane(panel, 20, false)
		panel.Width(430).Padding(22).Gap(12)
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "软件更新").FontSize(17).FontWeight(700).Grow(1)
			if v := mygo.App.Version(); v != "" {
				ui.Text(c, "v"+v).FontSize(11.5).TextColor(ink.faint)
			}
		})
		switch {
		case u.checking:
			ui.Text(c, "正在检查新版本…").FontSize(13).TextColor(ink.muted)
		case u.downloading:
			if u.up != nil {
				ui.Text(c, "新版本 "+u.up.Version).FontSize(20).FontWeight(700)
			}
			a.updateNotes(c, u.up, ink)
			a.updateProgress(c, ink)
		case u.ready:
			ui.Text(c, "新版本已经装好，重启后生效。").FontSize(13).TextColor(ink.muted).LineHeight(1.5)
		case u.err != "":
			ui.Text(c, u.err).FontSize(13).TextColor(ink.muted).LineHeight(1.5)
		case u.up != nil:
			ui.Text(c, "新版本 "+u.up.Version).FontSize(20).FontWeight(700)
			if !u.up.Date.IsZero() {
				ui.Text(c, u.up.Date.Format("2006年1月2日")).FontSize(11.5).TextColor(ink.faint)
			}
			a.updateNotes(c, u.up, ink)
		default:
			ui.Text(c, "已是最新版本").FontSize(15).FontWeight(600)
			ui.Text(c, "当前版本 "+mygo.App.Version()).FontSize(12).TextColor(ink.muted)
		}
		ui.Row(c).Gap(10).AlignItems(ui.Center).Justify(ui.End).Children(func() {
			if u.downloading {
				label := "正在下载…"
				if u.total > 0 {
					label = fmt.Sprintf("正在下载 %d%%", int(float64(u.got)*100/float64(u.total)))
				}
				ui.Text(c, label).FontSize(12).TextColor(ink.muted).FontFeatures("tnum")
				return
			}
			later := func(key, label string) {
				if pillButton(c.Key(key), nil, label, p.hover.Over(p.bg), p.text).Clicked() {
					u.open = false
				}
			}
			switch {
			case u.ready:
				later("later", "稍后")
				if pillButton(c.Key("relaunch"), nil, "立即重启", p.accent, p.onAccent).Clicked() {
					relaunchApp()
				}
			case u.up != nil && u.err == "":
				later("later", "稍后")
				if pillButton(c.Key("install"), nil, "下载并安装", p.accent, p.onAccent).Clicked() {
					a.installUpdate()
				}
			case u.err != "":
				// Neither GitHub nor its mirrors: the installers wait on a
				// cloud drive reachable from anywhere.
				if manualDownloadURL != "" && pillButton(c.Key("manual"), iconDownload, "从夸克网盘下载", p.hover.Over(p.bg), p.text).Clicked() {
					mygo.Shell.OpenExternal(manualDownloadURL)
				}
				if pillButton(c.Key("retry"), nil, "重试", p.accent, p.onAccent).Clicked() {
					a.checkForUpdates()
				}
				later("close", "关闭")
			default:
				later("close", "关闭")
			}
		})
	})
}

// updateNotes shows the release notes of up, the Markdown they come in
// tamed to the lines the pane shows.
func (a *app) updateNotes(c *ui.Context, up *mygo.Update, ink paneInk) {
	if up == nil {
		return
	}
	lines := updateNotesLines(up.Notes)
	// The notes repeat the version in a heading, which the pane says.
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == strings.TrimSpace(up.Version) {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return
	}
	height := min(float32(len(lines))*21+28, 210)
	ui.Scroll(c).Height(height).Padding(12, 14).Radius(14).Background(ink.hover).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			for _, line := range lines {
				ui.Text(c, line).FontSize(12.5).TextColor(ink.muted).LineHeight(1.5)
			}
		})
	})
}

// updateProgress draws how far the download of the update is.
func (a *app) updateProgress(c *ui.Context, ink paneInk) {
	frac := float32(0)
	if a.update.total > 0 {
		frac = min(float32(a.update.got)/float32(a.update.total), 1)
	}
	bar := ui.Box(c).Height(6).Radius(3).Background(ink.press)
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		if frac <= 0 {
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * frac, H: r.H}, a.pal.accent, 3)
	})
}

// updateNotesLines turns the Markdown of release notes into the lines the
// pane shows: headings without their hashes, bullets with a dot.
func updateNotesLines(notes string) []string {
	var out []string
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "![") {
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = "· " + strings.TrimSpace(line[2:])
		}
		out = append(out, line)
		if len(out) == 60 {
			break
		}
	}
	return out
}
