package platform

// Paster is the intent "paste into the app that had focus before the palette opened". The caller has
// already placed the content on the clipboard; the port sends the OS paste gesture to the frontmost app.
//
// Per-OS contract: Linux X11 sends Ctrl+V with xdotool and reports ErrUnsupported when xdotool is missing
// or the session is Wayland; macOS sends Command+V through osascript and needs the Accessibility permission
// (UNTESTED, see real/paster_darwin.go); Windows reports ErrUnsupported until a SendInput helper exists. The
// port waits for the palette to hide before it types, through a callback the composition root supplies, so
// the keys never reach the palette. When expectedAppID is set and another app is frontmost, the port returns
// ErrConflict and types nothing, so text never lands in the wrong window.
type Paster interface {
	// PasteIntoFrontmost sends the paste gesture to the frontmost app if it is expectedAppID, or to any
	// app when expectedAppID is "".
	PasteIntoFrontmost(expectedAppID string) error
}
