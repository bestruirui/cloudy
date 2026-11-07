package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The dialogs share one head, one foot and one pair of buttons, the
// settings': a title of 20 with what it is about under it in 12.5 and a
// close button on the right; and at the foot, capsules of 32, the one that
// does what the dialog is for in the accent.

// dialogHeader is the head of a dialog: its title, a word under it if sub
// says one, and a button that closes it, if close is given.
func dialogHeader(c *ui.Context, ink paneInk, title, sub string, close func()) {
	ui.Row(c).Padding(22, 20, 14, 28).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, title).FontSize(20).FontWeight(700)
			if sub != "" {
				ui.Text(c, sub).FontSize(12.5).TextColor(ink.muted).LineHeight(1.4)
			}
		})
		if close != nil {
			if iconButton(c.Key("dialog-close"), iconX, 16, ink.muted, "关闭").Clicked() {
				close()
			}
		}
	})
}

// dialogFooter is the foot of a dialog, its buttons on the right.
func dialogFooter(c *ui.Context, buttons func()) {
	ui.Row(c).Padding(14, 28, 22, 28).Gap(10).AlignItems(ui.Center).Justify(ui.End).Children(buttons)
}

// primaryButton is the button that does what a dialog is for: a capsule in
// the accent.
func primaryButton(c *ui.Context, label string, icon *ui.SVG, pal palette, disabled bool) ui.Element {
	return fillButton(c, label, icon, pal.accent, pal.onAccent, disabled)
}

// dangerButton is the button that deletes what a dialog names: red, whatever
// the accent.
func dangerButton(c *ui.Context, label string, icon *ui.SVG, disabled bool) ui.Element {
	return fillButton(c, label, icon, dangerColor, white, disabled)
}

// fillButton is a capsule of 32 filled with bg.
func fillButton(c *ui.Context, label string, icon *ui.SVG, fill, fg ui.Color, disabled bool) ui.Element {
	b := ui.ButtonBase(c).Height(32).PaddingX(18).Gap(6).Radius(999).AlignItems(ui.Center).Shrink(0).
		FocusRing(false).Label(label).Disabled(disabled)
	bg := fill
	if b.Hovered() && !disabled {
		bg = bg.Mix(white, 0.12)
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond})).TextColor(fg)
	if disabled {
		b.Opacity(0.45)
	}
	pose := ui.Pose{}
	if b.Pressed() && !disabled {
		pose.Scale = 0.96
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14)
		}
		ui.Text(c, label).FontSize(13.5).FontWeight(600).SingleLine()
	})
	return b
}
