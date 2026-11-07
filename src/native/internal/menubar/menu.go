package menubar

import "github.com/pablopunk/nvm/native/internal/platform"

// Menu is what the app's menu shows beyond its actions: the palette shortcut, the update row, and the platform the menu
// is drawn for. 1.x draws shortcuts on every platform but Linux, and icons on macOS only.
type Menu struct {
	GOOS        string
	Accelerator string
	Update      *UpdateRow
}

// shortcut is the palette shortcut the Open row shows. It is empty when no shortcut is set or the platform draws none.
func (m Menu) shortcut() string {
	if m.GOOS == "linux" {
		return ""
	}
	return m.Accelerator
}

// updateOrCheckRow is the update row while an update is found, downloading, installing or failed to install. Otherwise
// it is Check for Updates…, because 1.x shows one of the two in that place, never both.
func (m Menu) updateOrCheckRow(a Actions, onUI func(fn func())) platform.MenuItem {
	if m.Update == nil {
		return row("check-updates", "Check for Updates…", a.CheckForUpdates, onUI)
	}
	return m.Update.menuRow(m.GOOS, onUI)
}

// Rows is the app's menu, in the 1.x order: Open Nevermind with the palette shortcut, a divider, the update row or
// Check for Updates…, Settings…, a divider, Quit Nevermind. Each action runs on the UI thread when onUI is set, and
// directly otherwise.
func Rows(a Actions, onUI func(fn func()), m Menu) []platform.MenuItem {
	open := row("open", "Open Nevermind", a.Open, onUI)
	open.Accelerator = m.shortcut()
	return []platform.MenuItem{
		open,
		{Separator: true},
		m.updateOrCheckRow(a, onUI),
		row("settings", "Settings…", a.Settings, onUI),
		{Separator: true},
		row("quit", "Quit Nevermind", a.Quit, onUI),
	}
}

// row is one menu entry whose click runs fn on the UI thread.
func row(id, label string, fn func(), onUI func(fn func())) platform.MenuItem {
	return platform.MenuItem{ID: id, Label: label, Action: UIAction(fn, onUI)}
}

// UIAction wraps fn so it runs through onUI. A nil fn stays nil, which the port treats as no action.
func UIAction(fn func(), onUI func(fn func())) func() {
	if fn == nil {
		return nil
	}
	if onUI == nil {
		return fn
	}
	return func() { onUI(fn) }
}
