package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openSettings shows the settings over the main window, where its glass has
// the app to show through: a window of its own would have nothing under it.
func (a *app) openSettings() {
	a.measureCache()
	a.settingsOpen = true
	a.showMain()
	a.redraw()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	a.settings.applyAppearance()
	if a.notch != nil {
		a.notch.setEnabled(a.settings.Notch)
	}
	a.redraw()
}

// settingsPanel is the settings, a pane over the window while they are open.
func (a *app) settingsPanel(c *ui.Context) {
	w, h := c.Size()
	ink := a.inkOn(false)
	a.dialog(c, "settings", 0.3, &a.settingsOpen, func(panel ui.Element) {
		// It grows from the corner of who listens, whose row opens it.
		panel.Width(min(640, w-48)).Height(max(h-72, 320)).TextColor(ink.text).Label("设置").TransformOrigin(0, 1)
		a.pane(panel, 26, false)
		panel.Children(func() {
			ui.Row(c).Height(60).Padding(0, 16, 0, 28).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(20).FontWeight(700).Grow(1)
				if iconButton(c.Key("close"), iconX, 16, ink.muted, "关闭").Clicked() {
					a.settingsOpen = false
				}
			})
			a.settingsBody(c)
		})
	})
}

// settingsBody lists the settings, what is seen first, first: who listens,
// and how the app looks.
func (a *app) settingsBody(c *ui.Context) {
	ink := a.inkOn(false)
	s := &a.settings

	ui.Scroll(c.Key("settings-list")).Grow(1).Padding(0, 24, 28, 24).Gap(8).Children(func() {
		card := func(rows func()) {
			ui.Column(c).Radius(20).Background(ink.hover).Children(rows)
		}
		group := func(title string, rows func()) {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(ink.muted).Margin(14, 4, 4, 4)
			card(rows)
		}
		row := func(label, desc string, control func()) {
			ui.Row(c).Padding(12, 16).Gap(16).AlignItems(ui.Center).MinHeight(52).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, label).FontSize(13.5).FontWeight(500)
					if desc != "" {
						ui.Text(c, desc).FontSize(12).TextColor(ink.muted).LineHeight(1.4)
					}
				})
				control()
			})
		}
		// A slider of what the switch above it has on, from 0 to 1.
		slider := func(label, desc string, v *float64, on bool) {
			row(label, desc, func() {
				a.settingSlider(c.Key(label), label, v, on, ink)
			})
		}

		a.accountCard(c, card, row, ink)

		group("视觉效果", func() {
			row("液态玻璃播放栏", "底部播放栏化作悬浮的玻璃胶囊，透出并折射其下滚动的内容", func() {
				if ui.Switch(c.Key("liquid-glass"), &s.LiquidGlass).Label("液态玻璃播放栏").Changed() {
					a.changed()
				}
			})
			slider("玻璃透明度", "越高越能看清背后的内容，文字会更难读", &s.GlassClear, s.LiquidGlass)
			slider("折射强度", "胶囊边缘弯折背后内容的程度", &s.GlassBend, s.LiquidGlass)
			row("界面动画", "展开播放页、切换页面和弹出菜单的动画；精简时只保留淡入淡出", func() {
				modes := []string{"", "lite", "off"}
				i := 0
				for k, m := range modes {
					if m == s.Motion {
						i = k
					}
				}
				if a.segmented(c.Key("motion"), &i, "完整", "精简", "关闭") {
					s.Motion = modes[i]
					a.changed()
				}
			})
			row("播放栏封面旋转", "播放时胶囊里的封面像黑胶一样缓慢转动；会多用一些电量", func() {
				if ui.Switch(c.Key("bar-spin"), &s.BarSpin).Label("播放栏封面旋转").Changed() {
					a.changed()
				}
			})
			row("界面玻璃", "菜单、弹窗、侧栏和这个设置面板使用玻璃材质；关闭后为纯色，显卡占用更低", func() {
				if ui.Switch(c.Key("glass-panels"), &s.GlassPanels).Label("界面玻璃").Changed() {
					a.changed()
				}
			})
			row("播放页微光", "当前歌词、进度条与音量带一层柔和的微光，唱长音时轻轻亮起", func() {
				if ui.Switch(c.Key("np-light"), &s.NpLight).Label("播放页微光").Changed() {
					a.changed()
				}
			})
			slider("微光强度", "", &s.GlowLevel, s.NpLight)
			row("烟雾背景", "播放页的背景化作封面色调的烟雾，缓缓升起，随低音起伏；会多用一些电量", func() {
				if ui.Switch(c.Key("fog"), &s.Fog).Label("烟雾背景").Changed() {
					a.changed()
				}
			})
			row("进度条烟雾特效", "播放时进度条化作拖着烟雾的光点，随音乐起伏；会多用一些电量", func() {
				if ui.Switch(c.Key("smoke"), &s.Smoke).Label("进度条烟雾特效").Changed() {
					a.changed()
				}
			})
		})

		group("外观", func() {
			row("外观", "", func() {
				modes := []string{"system", "light", "dark"}
				i := 0
				for k, m := range modes {
					if m == s.Appearance {
						i = k
					}
				}
				if a.segmented(c.Key("appearance"), &i, "跟随系统", "浅色", "深色") {
					s.Appearance = modes[i]
					a.changed()
				}
			})
			row("主题色跟随封面", "按钮和高亮使用当前歌曲封面的颜色，而不是网易云红", func() {
				if ui.Switch(c.Key("accent-cover"), &s.AccentCover).Label("主题色跟随封面").Changed() {
					a.changed()
				}
			})
			row("字体", "界面、歌词与评论使用的字体", func() {
				fonts := a.fontChoices()
				a.loadFontFiles()
				labels := make([]string, len(fonts))
				chosen := fontOf(s.Font).label
				for i, f := range fonts {
					labels[i] = f.label
				}
				own := func(label string) string {
					for _, f := range fonts {
						if f.label == label && f.id != "" {
							return f.family
						}
					}
					return defaultFontFamily()
				}
				if a.paneSelect(c.Key("font"), "字体", &chosen, labels, own) {
					for _, f := range fonts {
						if f.label == chosen {
							s.Font = f.id
						}
					}
					a.loadFont()
					a.changed()
				}
			})
		})

		// Only on macOS: elsewhere the icon is always in the notification
		// area.
		if runtime.GOOS == "darwin" {
			group("通用", func() {
				row("应用图标", "显示在菜单栏时，关闭窗口后可从菜单栏图标打开", func() {
					labels := make([]string, len(appIcons))
					chosen := appIcons[0].label
					for i, ic := range appIcons {
						labels[i] = ic.label
						if ic.id == s.AppIcon {
							chosen = ic.label
						}
					}
					if a.paneSelect(c.Key("app-icon"), "应用图标", &chosen, labels) {
						for _, ic := range appIcons {
							if ic.label == chosen {
								s.AppIcon = ic.id
							}
						}
						a.changed()
						a.changeAppIcon()
					}
				})
			})
		}

		// Only on macOS, the one with a notch player.
		if a.notch != nil {
			group("刘海播放器", func() {
				row("显示刘海播放器", "在屏幕顶部的刘海处显示迷你播放器，鼠标悬停即可展开", func() {
					if ui.Switch(c.Key("notch"), &s.Notch).Label("显示刘海播放器").Changed() {
						a.changed()
					}
				})
				row("播放时显示实时活动", "在刘海两侧显示封面与音频波形，可能遮挡刘海旁的菜单栏图标", func() {
					if ui.Switch(c.Key("activity"), &s.NotchActivity).Label("播放时显示实时活动").Disabled(!s.Notch).Changed() {
						a.changed()
					}
				})
				row("在无刘海的屏幕上常驻显示", "关闭时，将鼠标移至屏幕顶部中央即可唤出，不会遮挡菜单栏", func() {
					if ui.Switch(c.Key("virtual"), &s.NotchVirtual).Label("在无刘海的屏幕上常驻显示").Disabled(!s.Notch).Changed() {
						a.changed()
					}
				})
			})
		}

		group("播放", func() {
			row("音质", "无损与 Hi-Res 需要黑胶 VIP，没有时播放所能播放的最高音质；下一首歌曲开始生效", func() {
				labels := make([]string, len(qualityLevels))
				chosen := qualityLevels[2].label
				for i, q := range qualityLevels {
					labels[i] = q.label
					if q.level == s.Quality {
						chosen = q.label
					}
				}
				if a.paneSelect(c.Key("quality"), "音质", &chosen, labels) {
					for _, q := range qualityLevels {
						if q.label == chosen {
							s.Quality = q.level
						}
					}
					a.changed()
				}
			})
			row("间奏时显示热评", "前奏、间奏与尾奏时，歌词处换成这首歌的热门评论", func() {
				if ui.Switch(c.Key("hot-comments"), &s.HotComments).Label("间奏时显示热评").Changed() {
					a.changed()
				}
			})
			a.sleepRow(c, row)
			keys := "在其他软件里也能用：Ctrl+Alt+←→ 切歌，Ctrl+Alt+↑↓ 调音量"
			if a.playKey != "" {
				keys = "在其他软件里也能用：" + a.playKey + " 播放暂停，Ctrl+Alt+←→ 切歌，Ctrl+Alt+↑↓ 调音量"
			}
			if a.keysTaken > 0 {
				keys += fmt.Sprintf("；有 %d 组被其他软件占用", a.keysTaken)
			}
			row("全局快捷键", keys, func() {
				if ui.Switch(c.Key("global-keys"), &s.GlobalKeys).Label("全局快捷键").Changed() {
					a.applyGlobalKeys()
					a.changed()
				}
			})
			row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
				if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
					a.pl.SetVolume(a.volume)
					a.changed()
				}
			})
		})

		if a.captionsShown() {
			group("播客", func() { a.captionSettings(row, c) })
		}

		group("缓存", func() {
			row("歌曲缓存", "听过的歌曲保存在本机，再次播放无需等待，也不耗流量；"+a.cacheUsage(), func() {
				labels := make([]string, len(cacheLimits))
				chosen := cacheLimits[2].label
				for i, l := range cacheLimits {
					labels[i] = l.label
					if l.mb == s.CacheMB {
						chosen = l.label
					}
				}
				if a.paneSelect(c.Key("cache"), "歌曲缓存", &chosen, labels) {
					for _, l := range cacheLimits {
						if l.label == chosen {
							s.CacheMB = l.mb
						}
					}
					a.changed()
					a.applyCacheLimit()
				}
			})
			row("清除缓存", "删除保存在本机的全部歌曲", func() {
				if ui.Button(c.Key("clear-cache"), "清除").Disabled(a.cacheUsed <= 0).Clicked() {
					a.clearCache()
				}
			})
		})

		group("关于", func() { a.aboutCard(c, ink) })

		if a.updates {
			group("更新", func() {
				desc := "每天检查一次是否有新版本，只提醒，不会自动安装"
				if t := lastUpdateCheck(); !t.IsZero() {
					desc += "；上次检查：" + commentTime(t, time.Now())
				}
				row("提醒新版本", desc, func() {
					on := updateChecks()
					if ui.Switch(c.Key("update-checks"), &on).Label("提醒新版本").Changed() {
						setUpdateChecks(on)
					}
				})
				row("当前版本 "+mygo.App.Version(), "由 nya 维护，新版本来自 GitHub 的 nya-owo/mygo-nya", func() {
					if ui.Button(c.Key("check-updates"), "检查更新").Clicked() {
						checkForUpdates()
					}
				})
			})
		}

		group("反馈", func() { a.feedbackCard(c, ink) })
	})
}

// accountCard heads the settings with who listens.
func (a *app) accountCard(c *ui.Context, card func(func()), row func(string, string, func()), ink paneInk) {
	card(func() {
		if a.profile == nil {
			row("未登录", "登录后，每日推荐与歌单将根据你的口味生成", func() {
				if ui.PrimaryButton(c.Key("login"), "扫码登录").Clicked() {
					a.settingsOpen = false
					a.openLogin()
				}
			})
			return
		}
		prof := a.profile
		ui.Row(c).Padding(14, 16).Gap(14).AlignItems(ui.Center).Children(func() {
			a.avatar(c, prof.AvatarURL, 48, ink.press)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, prof.Nickname).FontSize(16).FontWeight(700).SingleLine()
				ui.Text(c, "已登录网易云音乐").FontSize(12).TextColor(ink.muted)
			})
			if ui.Button(c.Key("logout"), "退出登录").Clicked() {
				a.signOut()
			}
		})
	})
}

// settingSlider is a slider of a setting, from 0 to 1, with how far it is
// said beside it. It keeps the settings as the pointer lets go, rather than
// at each step of a drag.
func (a *app) settingSlider(c *ui.Context, label string, v *float64, on bool, ink paneInk) {
	opacity := float32(1)
	if !on {
		opacity = 0.4
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Opacity(opacity).Children(func() {
		s := ui.SliderBase(c.Key("slider"), v, 0, 1).Width(150).Height(22).Step(0.01).FocusRing(false).Label(label).Disabled(!on).Cursor(ui.CursorPointer)
		if s.Changed() {
			a.settingsDirty = true
			a.redraw()
		}
		held := s.Pressed()
		if a.settingsDirty && !held {
			a.settingsDirty = false
			a.settings.save()
		}
		thick := float32(4)
		if s.Hovered() || held {
			thick = 5
		}
		thick = s.AnimateTo("thick", thick, ui.Snappy)
		at, accent := float32(*v), a.pal.accent
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			y := r.Y + (r.H-thick)/2
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, ink.press, thick/2)
			p.Fill(ui.Rect{X: r.X, Y: y, W: max(thick, r.W*at), H: thick}, accent, thick/2)
			const knob = 16
			k := ui.Rect{X: r.X + (r.W-knob)*at, Y: r.Y + (r.H-knob)/2, W: knob, H: knob}
			p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.3))
			p.Fill(k, white, knob/2)
		})
		ui.Text(c, fmt.Sprintf("%d%%", int(*v*100+0.5))).FontSize(12).FontFeatures("tnum").TextColor(ink.muted).Width(36).TextAlign(ui.End)
	})
}
