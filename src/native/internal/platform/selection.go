package platform

// SelectedText is the intent "send the copy gesture to the app in front, so the text it has selected lands on the
// clipboard". The port only sends the gesture: the caller gives the keyboard to the source app first and reads
// the clipboard afterwards. Capability selection.copy reports whether the OS can send it.
//
// Per-OS contract: Linux X11 sends Ctrl+C with xdotool, like the paste port, and reports ErrUnsupported on Wayland
// or without xdotool; macOS sends Command+C through System Events with osascript and needs the Accessibility
// permission (UNTESTED, see real/selected_text_darwin.go); other systems report ErrUnsupported and the capability
// is omitted. When expectedAppID is set and another app is frontmost, the port returns ErrConflict and sends nothing.
type SelectedText interface {
	// CopySelection sends the copy gesture to the frontmost app if it is expectedAppID, or to any app when
	// expectedAppID is "".
	CopySelection(expectedAppID string) error
}
