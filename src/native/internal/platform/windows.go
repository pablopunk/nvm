package platform

import (
	"context"
	"fmt"
)

// WindowFrame is a window position and size in desktop points. Origins may be negative for displays
// placed left of or above the primary display.
type WindowFrame struct {
	X, Y, Width, Height float64
}

// WindowDisplay is one monitor as the window system reports it. WorkArea excludes the menu bar, Dock,
// and panels; it is what window layouts fill.
type WindowDisplay struct {
	ID       string
	Primary  bool
	Frame    WindowFrame
	WorkArea WindowFrame
}

// WindowApp is a running application that may own windows. BundleID is the app's stable identifier
// (macOS bundle identifier, X11 WM_CLASS class).
type WindowApp struct {
	PID      int
	BundleID string
	Name     string
}

// WindowRecord is one window as the window system reports it. Key is opaque to callers, unique among
// all windows, and always starts with "<PID>:" so per-process state can be dropped. ZIndex is 0 for the
// front-most window and grows toward the back; windows the system does not stack use a large value.
type WindowRecord struct {
	Key        string
	PID        int
	BundleID   string
	AppName    string
	Title      string
	Frame      WindowFrame
	DisplayID  string
	FullScreen bool
	Minimized  bool
	Movable    bool
	Resizable  bool
	ZIndex     int
}

// WindowAccess is whether the app may read and move other apps' windows.
type WindowAccess int

const (
	// WindowAccessUnknown means the system could not say.
	WindowAccessUnknown WindowAccess = iota
	// WindowAccessAllowed means windows can be read and moved.
	WindowAccessAllowed
	// WindowAccessDenied means the user has not granted the access the system requires.
	WindowAccessDenied
)

// WindowWrite asks for a window to be placed. PositionOnly moves the origin and keeps the size, used to
// pull back a window that the app resized past its display edge.
type WindowWrite struct {
	PID          int
	Key          string
	Frame        WindowFrame
	PositionOnly bool
}

// WindowErrorCode names why a window operation could not be carried out.
type WindowErrorCode string

const (
	// WindowErrorDenied means the system refused access.
	WindowErrorDenied WindowErrorCode = "denied"
	// WindowErrorNoWindow means the app has no window to act on.
	WindowErrorNoWindow WindowErrorCode = "no-window"
	// WindowErrorTargetLost means the window existed but is gone now.
	WindowErrorTargetLost WindowErrorCode = "target-lost"
	// WindowErrorUnsupported means this system cannot perform the operation.
	WindowErrorUnsupported WindowErrorCode = "unsupported"
	// WindowErrorUnapplied means the system accepted the call but the window did not change.
	WindowErrorUnapplied WindowErrorCode = "unapplied"
	// WindowErrorFailed means an unexpected failure in the window system.
	WindowErrorFailed WindowErrorCode = "error"
)

// WindowError is returned by WindowManager methods for the failures callers must tell apart. Any other
// error is an unexpected failure.
type WindowError struct {
	Code    WindowErrorCode
	Message string
}

// Error describes the failure for logs.
func (e *WindowError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("window system: %s", e.Code)
	}
	return fmt.Sprintf("window system: %s: %s", e.Code, e.Message)
}

// WindowManager is the intent "read and place other apps' windows". The OS mechanism (Accessibility on
// macOS, EWMH on X11) stays behind it. Methods return *WindowError for the failures named by its codes.
// Frames are read back after every write, so callers can verify what the window actually reached.
// Platforms without the capability return no implementation and omit the capability; see
// CapabilityWindowManagement.
type WindowManager interface {
	// Access reports whether windows can be read and moved right now.
	Access(ctx context.Context) (WindowAccess, error)
	// Displays lists the connected monitors with their work areas.
	Displays(ctx context.Context) ([]WindowDisplay, error)
	// RunningApps lists the apps that may own windows.
	RunningApps(ctx context.Context) ([]WindowApp, error)
	// Windows lists the windows of one process, with ZIndex set.
	Windows(ctx context.Context, pid int) ([]WindowRecord, error)
	// AllWindows lists the windows of every app, with ZIndex set.
	AllWindows(ctx context.Context) ([]WindowRecord, error)
	// FocusedWindow returns the focused window of one process, or WindowErrorNoWindow.
	FocusedWindow(ctx context.Context, pid int) (WindowRecord, error)
	// Frontmost returns the app that is in front, if any.
	Frontmost(ctx context.Context) (WindowApp, bool, error)
	// SetFrame places a window and returns the frame it reached on readback.
	SetFrame(ctx context.Context, write WindowWrite) (WindowFrame, error)
	// SetFullScreen enters or leaves full screen and returns the frame reached on readback.
	SetFullScreen(ctx context.Context, pid int, key string, fullScreen bool) (WindowFrame, error)
	// Raise brings a window to the front and makes it key.
	Raise(ctx context.Context, pid int, key string) error
}
