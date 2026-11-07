package app

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type shortcutEditor struct {
	open      bool
	recording string
	capture   ui.Handle
	err       string
	scroll    ui.ScrollState
}

func (a *app) openShortcutEditor() {
	a.shortcutSetup.open = true
	a.shortcutSetup.recording, a.shortcutSetup.err = "", ""
	a.shortcutSetup.scroll = ui.ScrollState{}
	a.redraw()
}

func (a *app) shortcutDisplay(act shortcutAction) string {
	custom := strings.TrimSpace(a.settings.Shortcuts[act.id])
	if !a.settings.GlobalKeys {
		if custom != "" {
			return keyLabel(custom)
		}
		if len(act.keys) > 0 {
			return keyLabel(act.keys[0])
		}
		return "未设置"
	}
	if custom != "" {
		if a.shortcutKeys[act.id] == "" {
			return keyLabel(custom) + "（未注册）"
		}
		return keyLabel(custom)
	}
	if key := a.shortcutKeys[act.id]; key != "" {
		return keyLabel(key)
	}
	if len(act.keys) > 0 {
		return keyLabel(act.keys[0]) + "（默认，未注册）"
	}
	return "未设置"
}

func shortcutDefaults(act shortcutAction) string {
	if len(act.keys) == 0 {
		return "默认未设置"
	}
	// Each default, in the order tried: the next where another app holds
	// the one before.
	labels := make([]string, len(act.keys))
	for i, k := range act.keys {
		labels[i] = keyLabel(k)
	}
	return "默认 " + strings.Join(labels, " / ")
}

// shortcutEditorDialog lists the global actions and records a new key for
// one of them. Escape cancels the recording, not the dialog.
func (a *app) shortcutEditorDialog(c *ui.Context) {
	s := &a.shortcutSetup
	if !s.open {
		return
	}
	ink := a.inkOn(false)
	w, h := c.Size()
	a.dialog(c, "shortcut-editor", 0.35, &s.open, func(panel ui.Element) {
		panel.Width(min(620, w-80)).Height(min(700, h-56)).TextColor(ink.text)
		a.pane(panel, 26, false)
		panel.Children(func() {
			capture := ui.Box(c.Key("shortcut-capture")).Size(1, 1).Focusable().Bind(&s.capture).AutoFocus()
			capture.HandleInput(func(ev ui.InputEvent) bool { return a.recordShortcut(ev) })
			ui.Column(c).Grow(1).Padding(24).Gap(16).Children(func() {
				hint := "选择一个功能录制组合键；按 Esc 取消本次录制"
				if !a.settings.GlobalKeys {
					hint = "全局快捷键已关闭；可以先保存组合键，开启后生效"
				}
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
						ui.Text(c, "自定义全局快捷键").FontSize(20).FontWeight(700)
						ui.Text(c, hint).FontSize(12.5).TextColor(ink.muted)
					})
					if iconButton(c.Key("shortcut-close"), iconX, 16, ink.muted, "关闭").Clicked() {
						s.open = false
					}
				})
				ui.Scroll(c.Key("shortcut-list")).Grow(1).Shrink(1).TrackScroll(&s.scroll).Padding(8).Gap(8).
					Radius(18).Background(ink.hover.Alpha(0.05)).Border(0.5, ink.line).Children(func() {
					for _, act := range shortcutActions {
						recording := s.recording == act.id
						row := ui.Row(c).Height(56).Shrink(0).PaddingX(12).Gap(14).Radius(13).AlignItems(ui.Center)
						bg := ui.Transparent
						if row.Hovered() {
							bg = ink.hover
						}
						if recording {
							bg = ink.press
						}
						row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
						row.Children(func() {
							ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
								ui.Text(c, act.label).FontSize(13.5).FontWeight(600)
								ui.Text(c, shortcutDefaults(act)).FontSize(11.5).TextColor(ink.muted).SingleLine()
							})
							key := a.shortcutDisplay(act)
							keyColor := ink.text
							if recording {
								key, keyColor = "等待按键…", ink.muted
							}
							ui.Text(c, key).FontSize(12).FontFeatures("tnum").TextColor(keyColor).
								Width(180).TextAlign(ui.End).SingleLine().Shrink(0)
							label := "录制"
							if recording {
								label = "取消"
							}
							if pillButton(c.Key("record-"+act.id), nil, label, ink.text.Alpha(0.08), ink.text).Width(80).Justify(ui.Center).Clicked() {
								if recording {
									s.recording, s.err = "", ""
								} else {
									s.recording, s.err = act.id, ""
									s.capture.Focus(c)
								}
								a.redraw()
							}
						})
					}
				})
				if s.err != "" {
					ui.Text(c, s.err).FontSize(12).TextColor(ink.muted)
				}
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					if pillButton(c.Key("shortcut-reset"), iconReload, "恢复默认", ink.text.Alpha(0.08), ink.text).Clicked() {
						a.settings.Shortcuts = nil
						a.settings.save()
						a.applyGlobalKeys()
						s.recording, s.err = "", ""
						a.toast = "已恢复默认全局快捷键"
						a.redraw()
					}
					ui.Spacer(c)
					if pillButton(c.Key("shortcut-done"), iconCheck, "完成", a.pal.accent, a.pal.onAccent).Clicked() {
						s.open = false
					}
				})
			})
		})
	})
}

// recordShortcut takes the keys pressed while a row is recording.
func (a *app) recordShortcut(ev ui.InputEvent) bool {
	s := &a.shortcutSetup
	if s.recording == "" || ev.Kind != ui.InputKeyDown {
		return false
	}
	if ev.Repeat {
		return true
	}
	if ev.Key == ui.KeyEscape {
		s.recording, s.err = "", ""
		a.redraw()
		return true
	}
	keys, ok := shortcutFromEvent(ev)
	if !ok {
		s.err = "请按 Ctrl/Alt/Super 加一个键，或直接按 F1–F12"
		a.redraw()
		return true
	}
	var act shortcutAction
	for _, candidate := range shortcutActions {
		if candidate.id == s.recording {
			act = candidate
			break
		}
	}
	for _, other := range shortcutActions {
		if other.id == act.id {
			continue
		}
		used := strings.TrimSpace(a.settings.Shortcuts[other.id])
		if used == "" {
			used = a.shortcutKeys[other.id]
		}
		if used != "" && strings.EqualFold(used, keys) {
			s.err = "这个组合键已经用于「" + other.label + "」"
			a.redraw()
			return true
		}
	}
	// Held by another app (NetEase's own player holds Ctrl+Alt+M), it would
	// not work: the listener learns it now, not after saving.
	if a.settings.GlobalKeys && !strings.EqualFold(a.shortcutKeys[act.id], keys) {
		if err := registerShortcut(keys, func() {}); err != nil {
			s.err = "「" + keyLabel(keys) + "」已被其他软件占用，请换一个组合键"
			a.redraw()
			return true
		}
		unregisterShortcut(keys)
	}
	if a.settings.Shortcuts == nil {
		a.settings.Shortcuts = map[string]string{}
	}
	a.settings.Shortcuts[act.id] = keys
	a.settings.save()
	a.applyGlobalKeys()
	s.recording, s.err = "", ""
	switch {
	case !a.settings.GlobalKeys:
		a.toast = "已保存；打开全局快捷键后生效"
	case a.shortcutKeys[act.id] == "":
		a.toast = "已保存，但组合键被其他软件占用"
	default:
		a.toast = "已设置 " + act.label + "：" + keyLabel(keys)
	}
	a.redraw()
	return true
}

var shortcutKeyNames = map[ui.Key]string{
	ui.KeySpace: "Space", ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeyBackspace: "Backspace",
	ui.KeyDelete: "Delete", ui.KeyInsert: "Insert", ui.KeyHome: "Home", ui.KeyEnd: "End",
	ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown", ui.KeyLeft: "Left", ui.KeyRight: "Right",
	ui.KeyUp: "Up", ui.KeyDown: "Down",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".",
	ui.KeySlash: "/", ui.KeySemicolon: ";", ui.KeyQuote: "'", ui.KeyBracketLeft: "[",
	ui.KeyBracketRight: "]", ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
	ui.Key0: "0", ui.Key1: "1", ui.Key2: "2", ui.Key3: "3", ui.Key4: "4",
	ui.Key5: "5", ui.Key6: "6", ui.Key7: "7", ui.Key8: "8", ui.Key9: "9",
	ui.KeyA: "A", ui.KeyB: "B", ui.KeyC: "C", ui.KeyD: "D", ui.KeyE: "E",
	ui.KeyF: "F", ui.KeyG: "G", ui.KeyH: "H", ui.KeyI: "I", ui.KeyJ: "J",
	ui.KeyK: "K", ui.KeyL: "L", ui.KeyM: "M", ui.KeyN: "N", ui.KeyO: "O",
	ui.KeyP: "P", ui.KeyQ: "Q", ui.KeyR: "R", ui.KeyS: "S", ui.KeyT: "T",
	ui.KeyU: "U", ui.KeyV: "V", ui.KeyW: "W", ui.KeyX: "X", ui.KeyY: "Y", ui.KeyZ: "Z",
	ui.KeyF1: "F1", ui.KeyF2: "F2", ui.KeyF3: "F3", ui.KeyF4: "F4",
	ui.KeyF5: "F5", ui.KeyF6: "F6", ui.KeyF7: "F7", ui.KeyF8: "F8",
	ui.KeyF9: "F9", ui.KeyF10: "F10", ui.KeyF11: "F11", ui.KeyF12: "F12",
}

func shortcutFromEvent(ev ui.InputEvent) (string, bool) {
	name := shortcutKeyNames[ev.Key]
	if name == "" {
		return "", false
	}
	var mods []string
	if ev.Mods&ui.Ctrl != 0 {
		mods = append(mods, "Ctrl")
	}
	if ev.Mods&ui.Alt != 0 {
		mods = append(mods, "Alt")
	}
	if ev.Mods&ui.Shift != 0 {
		mods = append(mods, "Shift")
	}
	if ev.Mods&ui.Super != 0 {
		mods = append(mods, "Super")
	}
	strong := ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) != 0
	function := ev.Key >= ui.KeyF1 && ev.Key <= ui.KeyF12
	if !strong && !function {
		return "", false
	}
	if len(mods) == 0 {
		return name, true
	}
	return strings.Join(mods, "+") + "+" + name, true
}

// keyLabel shows a combination as the settings and menus write keys: the
// arrows as arrows, the rest as they are named.
func keyLabel(acc string) string {
	parts := strings.Split(acc, "+")
	for i, p := range parts {
		switch p {
		case "Left":
			parts[i] = "←"
		case "Right":
			parts[i] = "→"
		case "Up":
			parts[i] = "↑"
		case "Down":
			parts[i] = "↓"
		}
	}
	return strings.Join(parts, "+")
}
