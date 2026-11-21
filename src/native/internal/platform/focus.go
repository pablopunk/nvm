package platform

// FocusTarget names the window that had the keyboard, as the OS knows it: an opaque window ID and the process
// that owns the window. The process is what makes a restore safe: a window that changed owner is not restored.
type FocusTarget struct {
	Window string
	PID    int
}

// Focus is the intent "remember who had the keyboard and give it back": the palette takes the keyboard when it
// is summoned over the app the user was in, and returns it there when it hides without opening something else.
// Only X11 sessions on Linux support it today; elsewhere the OS (or MyGo's nonactivating panel) keeps the focus
// where it was, and the capability is omitted.
type Focus interface {
	// Active returns the window that has the keyboard, or ErrNotFound when no window does.
	Active() (FocusTarget, error)
	// Restore gives the keyboard back to target. It returns ErrNotFound when the window is gone, or when another
	// process owns it now.
	Restore(target FocusTarget) error
}
