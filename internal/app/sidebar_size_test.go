package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Dragging the sidebar's edge widens it, as far as leaves the page its
// room, and keeps the width; a double click makes it as it was.
func TestSidebarDrags(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1180, 780)
	tt.Frame()
	r, ok := tt.Find("调整侧边栏宽度")
	if !ok {
		t.Fatal("no edge to drag")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	for i := 1; i <= 10; i++ {
		tt.Move(x+float32(i*10), y)
		tt.Frame()
	}
	tt.Release(x+100, y)
	tt.Frame()
	if w := a.sideWidth(1180); w < sidebarWidth+90 || w > sidebarWidth+110 {
		t.Errorf("dragged 100 DIPs, the sidebar is %v wide", w)
	}
	if w := a.sideWidth(880); w > 880-pageMin {
		t.Errorf("in the narrowest window it leaves the page %v", 880-w)
	}
	a.settings.SidebarWidth = 1000
	if w := a.sideWidth(1600); w != sidebarMax {
		t.Errorf("dragged far, %v wide", w)
	}
}
