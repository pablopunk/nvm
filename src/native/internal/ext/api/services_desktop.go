package api

// Desktop groups the OS-facing services. Calls require their matching declared capability.
type Desktop interface {
	Keyboard() Keyboard
	Clipboard() SystemClipboard
	Selection() Selection
	Apps() Apps
	Menus() Menus
	// WindowManagement returns nil on hosts without window management. Check
	// System().HasCapability("window-management") before calling it.
	WindowManagement() WindowManagement
	Files() Files
	Shell() Shell
}

// TypeOptions tunes typing into the frontmost app.
type TypeOptions struct {
	DelayMs int64 `json:"delayMs,omitempty"`
}

// Keyboard types text into the frontmost app without touching the clipboard. Undo sends the undo gesture (Command+Z
// or Ctrl+Z) to the app the palette was summoned from, and needs a replacement made with Selection.ReplaceText in that
// window first. Undo requires the desktop.keyboard capability.
type Keyboard interface {
	TypeText(text string, options TypeOptions) error
	Undo() error
}
