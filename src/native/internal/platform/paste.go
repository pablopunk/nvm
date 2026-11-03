package platform

// Paster is the intent "paste into the app that had focus before the palette opened". The caller has
// already placed the content on the clipboard; the port sends the OS paste gesture to the frontmost app.
//
// Per-OS contract, implemented in WP1.6b: macOS posts Command+V through the helper and needs the
// Accessibility permission (capability clipboard.paste is omitted until it is granted); Windows sends
// Ctrl+V with SendInput after the palette hides; Linux X11 sends key events through the desktop's input
// tool; Linux Wayland reports ErrUnsupported. When expectedAppID is set and another app is frontmost,
// the port returns ErrConflict and types nothing, so text never lands in the wrong window.
type Paster interface {
	// PasteIntoFrontmost sends the paste gesture to the frontmost app if it is expectedAppID, or to any
	// app when expectedAppID is "".
	PasteIntoFrontmost(expectedAppID string) error
}
