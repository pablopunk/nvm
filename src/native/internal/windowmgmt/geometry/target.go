package geometry

// Rejection says why a focused window cannot be moved, before any write is attempted.
type Rejection int

const (
	// RejectNone means the window may be moved.
	RejectNone Rejection = iota
	// RejectTargetChanged means the focused window belongs to a different app than the target.
	RejectTargetChanged
	// RejectFullScreen means the window is full screen and the command does not accept that.
	RejectFullScreen
	// RejectMinimized means the window is minimized.
	RejectMinimized
)

// Message returns the palette copy for a rejection, or "" for RejectNone.
func (r Rejection) Message() string {
	switch r {
	case RejectTargetChanged:
		return MessageTargetWindowChanged
	case RejectFullScreen:
		return MessageFullScreen
	case RejectMinimized:
		return MessageMinimized
	}
	return ""
}

// RejectTargetWindow checks the focused window against the target app and the command's full-screen
// policy, in the same order as describeTargetWindowRejection in window-management-target.ts.
// sameApp reports whether the window belongs to the target's bundle.
func RejectTargetWindow(window WindowFrame, sameApp, allowFullScreen bool) Rejection {
	switch {
	case !sameApp:
		return RejectTargetChanged
	case window.IsFullScreen && !allowFullScreen:
		return RejectFullScreen
	case window.IsMinimized:
		return RejectMinimized
	}
	return RejectNone
}
