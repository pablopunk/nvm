package platform

// MenuItem is one row of a tray menu. A zero MenuItem with Separator set draws a divider. Action runs when
// the user picks the row, on a goroutine of the platform's choosing.
type MenuItem struct {
	ID        string
	Label     string
	Separator bool
	Disabled  bool
	Action    func()
}

// TrayOptions describes a menu bar or system tray icon. Icon is PNG; the empty Icon shows only the title.
type TrayOptions struct {
	ID      string
	Title   string
	ToolTip string
	Icon    []byte
}

// TrayHost is the intent "keep a small item in the menu bar or system tray", for the app's own icon and
// for extensions that show a live status item. Icons are optional on every OS; the title alone is a
// valid tray. Tray menus and icons on Linux depend on a running tray host and are verified in the rig.
type TrayHost interface {
	// NewTray creates a tray item. Its ID identifies it across launches where the OS supports that.
	NewTray(opts TrayOptions) (Tray, error)
}

// Tray is one menu bar or system tray item.
type Tray interface {
	// SetTitle changes the text shown next to (or instead of) the icon.
	SetTitle(title string)
	// SetMenu replaces the menu shown on click. An empty list removes the menu.
	SetMenu(items []MenuItem)
	// Close removes the item. Calls after Close do nothing.
	Close()
}
