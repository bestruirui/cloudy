package app

import (
	"fmt"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type settingsCategory int

const (
	settingsGeneral settingsCategory = iota
	settingsAppearance
	settingsPlayback
	settingsAbout
	settingsCategoryCount
)

var settingsCategories = [...]struct {
	label string
	icon  *ui.SVG
}{
	{"常规", iconSettings},
	{"外观", iconSun},
	{"播放", iconHeadset},
	{"关于", iconInfo},
}

// openSettings shows the settings over the main window, where its glass has
// the app to show through: a window of its own would have nothing under it.
func (a *app) openSettings() {
	a.measureCache()
	a.downloadPathDraft = a.downloadFolder()
	a.settingsOpen = true
	a.showMain()
	a.redraw()
}

// changed applies the settings everywhere and keeps them.
func (a *app) changed() {
	a.settings.save()
	if a.win != nil {
		// The theme source reaches the app's loop, which a view built in a
		// test has none of.
		a.settings.applyAppearance()
	}
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
		panel.Width(min(860, w-48)).Height(max(h-72, 320)).TextColor(ink.text).Label("设置").TransformOrigin(0, 1)
		a.pane(panel, 26, false)
		panel.Children(func() {
			ui.Row(c).Height(60).Padding(0, 16, 0, 28).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(20).FontWeight(700).Grow(1)
				if iconButton(c.Key("close"), iconX, 16, ink.muted, "关闭").Clicked() {
					a.settingsOpen = false
				}
			})
			ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Padding(0, 0, 12, 12).Children(func() {
				a.settingsSidebar(c)
				ui.Box(c).Width(0.5).Shrink(0).Background(ink.line)
				a.settingsBody(c)
			})
		})
	})
}

// settingsSidebar keeps the four destinations visible while their contents scroll.
func (a *app) settingsSidebar(c *ui.Context) {
	ink := a.inkOn(false)
	ui.Column(c.Key("settings-nav")).Width(148).Shrink(0).Padding(4, 12, 16, 4).Gap(6).Label("设置分类").Children(func() {
		for i, category := range settingsCategories {
			selected := a.settingsTab == settingsCategory(i)
			button := ui.ButtonBase(c.Key(category.label)).Height(42).PaddingX(12).Gap(10).Radius(14).
				AlignItems(ui.Center).Justify(ui.Start).Label("设置分类：" + category.label).Selected(selected)
			bg, color, weight := ui.Transparent, ink.muted, 500
			if selected {
				bg, color, weight = ink.press, ink.text, 600
			} else if button.Hovered() {
				bg, color = ink.hover, ink.text
			}
			button.Background(button.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).TextColor(color)
			button.Children(func() {
				ui.Icon(c, category.icon).Size(18, 18)
				ui.Text(c, category.label).FontSize(13.5).FontWeight(weight)
			})
			if button.Clicked() {
				a.settingsTab = settingsCategory(i)
			}
		}
	})
}

// settingsBody shows one category, keeping its scroll position between visits.
func (a *app) settingsBody(c *ui.Context) {
	ink := a.inkOn(false)
	s := &a.settings

	ui.Scroll(c.Key(a.settingsTab)).Label(settingsCategories[a.settingsTab].label+"设置项").TrackScroll(&a.settingsScroll[a.settingsTab]).Grow(1).Shrink(1).MinWidth(0).Padding(0, 24, 24, 24).Gap(8).Children(func() {
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

		ui.Text(c, settingsCategories[a.settingsTab].label).FontSize(21).FontWeight(700).Margin(4, 4, 4, 4)
		switch a.settingsTab {
		case settingsGeneral:
			a.accountCard(c, card, row, ink)

			group("快捷键", func() {
				keys := "在其他软件里也能用"
				if !s.GlobalKeys {
					keys = "已关闭"
				} else if a.keysTaken > 0 {
					keys = fmt.Sprintf("有 %d 组被其他软件占用", a.keysTaken)
				}
				row("全局快捷键", keys, func() {
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						if ui.Button(c.Key("shortcut-custom"), "自定义").Clicked() {
							a.openShortcutEditor()
						}
						if ui.Switch(c.Key("global-keys"), &s.GlobalKeys).Label("全局快捷键").Changed() {
							a.applyGlobalKeys()
							a.changed()
						}
					})
				})
			})

			group("启动与性能", func() {
				row("动画帧率", "旋转封面、播放页背景、逐字歌词等持续动画的帧率；滚动和页面切换始终跟随显示器", func() {
					labels := make([]string, len(frameRates))
					for i, r := range frameRates {
						labels[i] = r.label
					}
					chosen := frameRateLabel(s.FrameRate)
					if a.paneSelect(c.Key("frame-rate"), "动画帧率", &chosen, labels) {
						for _, r := range frameRates {
							if r.label == chosen {
								s.FrameRate = r.fps
							}
						}
						setFrameRate(s.FrameRate)
						a.changed()
					}
				})
				row("开机时启动", "登录 Windows 后自动打开 Cloudy", func() {
					on := a.openAtLogin()
					if ui.Switch(c.Key("open-at-login"), &on).Label("开机时启动").Changed() {
						a.setOpenAtLogin(on)
					}
				})
				row("开机启动时隐藏到托盘", "开机自动打开时不弹出窗口，从托盘图标打开", func() {
					if ui.Switch(c.Key("start-hidden"), &s.StartHidden).Label("开机启动时隐藏到托盘").Disabled(!a.openAtLogin()).Changed() {
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

			group("悬浮窗口", func() {
				row("桌面歌词", "在其他窗口之上显示当前歌词；鼠标移上去可以拖动、调字号和锁定", func() {
					if ui.Switch(c.Key("desk-lyrics"), &s.DeskLyrics).Label("桌面歌词").Changed() {
						a.applyDeskLyrics()
						a.changed()
					}
				})
				row("桌面歌词样式", "也可以把鼠标移到歌词上点“样式”切换", func() {
					labels := make([]string, len(deskStyles))
					chosen := deskStyles[0].label
					for i, d := range deskStyles {
						labels[i] = d.label
						if d.id == s.DeskStyle {
							chosen = d.label
						}
					}
					if a.paneSelect(c.Key("desk-style"), "桌面歌词样式", &chosen, labels) {
						for _, d := range deskStyles {
							if d.label == chosen {
								s.DeskStyle = d.id
							}
						}
						a.changed()
					}
				})
				row("锁定桌面歌词", "锁定后鼠标会穿过歌词，不会挡住下面的内容", func() {
					if ui.Switch(c.Key("desk-lock"), &s.DeskLock).Label("锁定桌面歌词").Changed() {
						a.applyDeskLyrics()
						a.changed()
					}
				})
				row("迷你窗口", "用一个置顶的小窗口代替主窗口；托盘菜单里也能打开", func() {
					if ui.Button(c.Key("mini"), "打开").Clicked() {
						a.settingsOpen = false
						a.showMini()
					}
				})
			})

		case settingsAppearance:
			group("主题与字体", func() {
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
				row("背景色", "窗口和侧栏的底色，按钮与高亮跟着它；深浅两套各自适配", func() {
					dark := c.Theme().Dark
					base := paletteFor(c.Theme())
					ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
						for _, ac := range accentColors {
							dot := ui.ButtonBase(c.Key("accent-"+ac.id)).Size(24, 24).Radius(999).
								FocusRing(false).Label("背景色：" + ac.label)
							fill := base.accent
							if ac.hex != "" {
								fill = accentFor(ui.Hex(ac.hex), dark)
							}
							dot.Background(fill)
							if s.Accent == ac.hex {
								dot.Border(2, ink.text)
							} else {
								dot.Border(1, ink.line)
							}
							if dot.Clicked() {
								s.Accent, s.AccentCover = ac.hex, false
								a.changed()
							}
						}
						own := s.Accent != ""
						for _, ac := range accentColors {
							if ac.hex == s.Accent {
								own = false
							}
						}
						wheel := ui.ButtonBase(c.Key("accent-custom")).Size(24, 24).Radius(999).
							FocusRing(false).Label("自定义主题色")
						wheel.Draw(func(pt *ui.Painter, r ui.Rect) {
							colorWheel(pt, r)
							if own {
								pt.Stroke(r, ink.text, 999, 2)
							} else {
								pt.Stroke(r, ink.line, 999, 1)
							}
						})
						if wheel.Clicked() {
							draft := base.accent
							if s.Accent != "" {
								draft = ui.Hex(s.Accent)
							}
							a.accentDraft = accentPick{open: true, color: draft}
							a.redraw()
						}
					})
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

		case settingsPlayback:
			group("音乐播放", func() {
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
				row("歌词翻译", "在歌词下方显示翻译；播放栏右侧的翻译按钮也能切换", func() {
					if ui.Switch(c.Key("lyric-trans"), &s.LyricTrans).Label("歌词翻译").Changed() {
						a.changed()
					}
				})
				a.sleepRow(c, row)
				row("音量均衡", "按网易云音乐测得的响度调整每首歌的音量，让歌曲听起来一样响", func() {
					if ui.Switch(c.Key("normalize"), &s.Normalize).Label("音量均衡").Changed() {
						a.pl.SetVolume(a.volume)
						a.changed()
					}
				})
				a.outputRow(c, row)
			})

			group("下载歌曲", func() {
				row("下载音质", "下载到本机的音质，可以和播放音质不同；无损与 Hi-Res 占用的空间大得多", func() {
					labels := make([]string, len(downloadLevels))
					chosen := downloadLevels[0].label
					for i, d := range downloadLevels {
						labels[i] = d.label
						if d.level == s.DownloadQuality {
							chosen = d.label
						}
					}
					if a.paneSelect(c.Key("download-quality"), "下载音质", &chosen, labels) {
						for _, d := range downloadLevels {
							if d.label == chosen {
								s.DownloadQuality = d.level
							}
						}
						a.changed()
					}
				})
				ui.Column(c).Padding(12, 16).Gap(10).Children(func() {
					ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
							ui.Text(c, "下载文件夹").FontSize(13.5).FontWeight(500)
							ui.Text(c, "留空放在音乐文件夹下的「Cloudy Downloads」；歌曲会写入歌名、歌手、封面和歌词").FontSize(12).TextColor(ink.muted).LineHeight(1.4)
						})
						if ui.Button(c.Key("download-open"), "打开").Clicked() {
							a.openDownloadFolder()
						}
					})
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						field := ui.Row(c).Grow(1).Height(34).PaddingX(10).Radius(8).
							Background(ink.press).Border(1, ink.line)
						field.Children(func() {
							path := ui.TextInputBase(c.Key("download-path"), &a.downloadPathDraft).Bind(&a.downloadBox).Grow(1).MinWidth(0).
								FontSize(12.5).Label("下载文件夹路径").Placeholder(`例如 D:\Music\Cloudy\Downloads`)
							if path.Submitted() {
								a.setDownloadFolder(a.downloadPathDraft)
							}
						})
						if ui.Button(c.Key("download-apply"), "应用").Clicked() {
							a.setDownloadFolder(a.downloadPathDraft)
						}
						if ui.Button(c.Key("download-browse"), "浏览…").Clicked() {
							a.pickDownloadFolder()
						}
					})
				})
			})

			group("播客", func() {
				a.podcastSpeedRow(c, row)
				if a.captionsShown() {
					a.captionSettings(row, c)
				}
			})

		case settingsAbout:
			group("软件信息", func() { a.aboutCard(c, ink) })

			if a.updates {
				group("更新", func() {
					desc := "每天检查一次是否有新版本"
					if s.UpdateChecked > 0 {
						desc += "；上次检查：" + commentTime(time.Unix(s.UpdateChecked, 0), time.Now())
					}
					row("提醒新版本", desc, func() {
						if ui.Switch(c.Key("update-checks"), &s.UpdateChecks).Label("提醒新版本").Changed() {
							a.changed()
						}
					})
					// Installing in the background goes with the checks,
					// which find what it installs.
					if s.UpdateChecks {
						row("自动安装更新", "在后台下载并装好新版本，不弹窗，下次打开软件时生效", func() {
							if ui.Switch(c.Key("update-downloads"), &s.UpdateInstall).Label("自动安装更新").Changed() {
								a.changed()
							}
						})
					}
					row("当前版本 "+mygo.App.Version(), "由 nya 维护，新版本来自 GitHub 的 nya-owo/cloudy", func() {
						if ui.Button(c.Key("check-updates"), "检查更新").Clicked() {
							a.checkForUpdates()
						}
					})
				})
			}

			group("反馈", func() { a.feedbackCard(c, ink) })
		}
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
