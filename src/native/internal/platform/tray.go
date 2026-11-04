package platform

// MenuItem is one row of a tray menu. A zero MenuItem with Separator set draws a divider. Action runs when
// the user picks the row, on a goroutine of the platform's choosing.
type MenuItem struct {
	ID        string
	Label     string
	Separator bool
	Disabled  bool
	Action    func()
	// Accelerator is the row's shortcut in canonical form ("Command+Alt+K"), shown at the row's right edge. Empty
	// shows none.
	Accelerator string
	// Icon is the image beside the label, nil for none.
	Icon *MenuIcon
	// Checkable draws a checkbox beside the label, ticked when Checked is set.
	Checkable bool
	Checked   bool
	// Submenu nests rows under this one; Action is then unused.
	Submenu []MenuItem
}

// MenuIcon is a PNG drawn beside a menu row. A template icon is a black glyph with transparency that the platform
// tints to the menu's text, as macOS does; other icons keep their colors.
type MenuIcon struct {
	PNG      []byte
	Template bool
}

// TrayOptions describes a menu bar or system tray icon. The icon is set with Tray.SetIcon; an item without one
// shows only its title.
type TrayOptions struct {
	ID      string
	Title   string
	ToolTip string
}

// TrayHost is the intent "keep a small item in the menu bar or system tray", for the app's own icon and
// for extensions that show a live status item. Icons are optional on every OS; the title alone is a
// valid tray. Tray menus and icons on Linux depend on a running tray host and are verified in the rig.
type TrayHost interface {
	// NewTray creates a tray item. Its ID identifies it across launches where the OS supports that.
	NewTray(opts TrayOptions) (Tray, error)
}

// PopoverOptions is the size of a tray item's popover, in DIPs.
type PopoverOptions struct {
	Width, Height int
}

// Popover is the window of a tray item's popover. It opens under the item, closes when the user presses outside it, and
// is driven as the palette drives its own window.
type Popover interface {
	// Update runs fn on the UI thread, then redraws. It is safe from any goroutine.
	Update(fn func())
	Show()
	Hide()
	IsVisible() bool
	Size() (width, height int)
	SetPosition(x, y int)
	// Focus gives the popover the keyboard.
	Focus()
	// IsFocused reports whether the popover has the keyboard.
	IsFocused() bool
}

// Tray is one menu bar or system tray item.
type Tray interface {
	// SetTitle changes the text shown next to (or instead of) the icon.
	SetTitle(title string)
	// SetMenu replaces the menu shown on click. An empty list removes the menu.
	SetMenu(items []MenuItem)
	// SetIcon replaces the icon with a PNG, at 16 points (32 pixels for Retina). A template icon is a black
	// glyph with transparency that macOS tints to the menu bar; other icons keep their colors. An empty PNG
	// leaves the icon as it is.
	SetIcon(png []byte, template bool) error
	// Dark reports whether the bar the icon sits on is dark, so a light or dark glyph can be picked.
	Dark() bool
	// OnAppearanceChange runs fn when the bar turns dark or light, on the platform's thread. Set the icon again there.
	OnAppearanceChange(fn func())
	// OnPrimaryClick runs fn when the icon is clicked. A click reaches fn only while the item has no attached menu.
	OnPrimaryClick(fn func())
	// OnSecondaryClick runs fn when the icon is right-clicked.
	OnSecondaryClick(fn func())
	// PopUpMenu shows items at the pointer as a context menu, without attaching them to the icon. No items shows nothing.
	PopUpMenu(items []MenuItem)
	// OpenPopover creates a hidden window of the given size under the icon, showing content: the native UI of a ui.View,
	// as the real platform takes it. A fake ignores it.
	OpenPopover(opts PopoverOptions, content any) (Popover, error)
	// Close removes the item. Calls after Close do nothing.
	Close()
}
