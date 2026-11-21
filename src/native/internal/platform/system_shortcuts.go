package platform

import "context"

// SystemShortcut is a keyboard shortcut the operating system currently has turned on for one of its own features. Key
// is spelled as an accelerator does (A, 4, Space, Up, F5).
type SystemShortcut struct {
	Name                         string
	Key                          string
	Command, Control, Alt, Shift bool
}

// SystemShortcuts is the intent "know which shortcuts the OS uses right now", so a shortcut is refused only when the
// OS would take it, and one the user turned off in the system settings stays available. It is nil where the OS has no
// reliable live source for them; callers then refuse nothing. A read may take a while, so callers cache and run it off
// the UI thread.
type SystemShortcuts interface {
	// Enabled lists the system shortcuts that are turned on. A failed read returns an error and no list.
	Enabled(ctx context.Context) ([]SystemShortcut, error)
}
