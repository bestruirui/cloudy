package app

import "github.com/egoist/mygo"

// openAtLogin reports whether the app starts as the listener logs in. The
// system keeps it (on Windows, a Run entry, which Task Manager can turn
// off), so it is asked once and remembered, rather than every frame.
func (a *app) openAtLogin() bool {
	if a.loginKnown == nil {
		on := mygo.App.OpenAtLogin()
		a.loginKnown = &on
	}
	return *a.loginKnown
}

// setOpenAtLogin has the app start as the listener logs in, or not.
func (a *app) setOpenAtLogin(on bool) {
	if err := mygo.App.SetOpenAtLogin(on); err != nil {
		a.toast = "无法更改开机启动：" + err.Error()
		a.redraw()
		return
	}
	a.loginKnown = &on
	a.redraw()
}

// startsHidden reports whether the window stays hidden as the app opens:
// started by the system at login, as the listener asked, with the tray
// icon to bring it back.
func (a *app) startsHidden() bool {
	return a.settings.StartHidden && mygo.App.WasOpenedAtLogin()
}
