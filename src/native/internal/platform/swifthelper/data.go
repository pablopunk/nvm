package swifthelper

import "github.com/pablopunk/nvm/native/internal/platform"

// Rect is a frame as the helper prints it, in points.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Platform converts a helper rectangle to the port's frame.
func (r Rect) Platform() platform.WindowFrame {
	return platform.WindowFrame{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// Display is one monitor from `displays`.
type Display struct {
	ID        string `json:"id"`
	IsPrimary bool   `json:"isPrimary"`
	Frame     Rect   `json:"frame"`
	WorkArea  Rect   `json:"workArea"`
}

// Window is one window from `list`, `focused` and `all`.
type Window struct {
	BundleID     string `json:"bundleId"`
	WindowKey    string `json:"windowKey"`
	Title        string `json:"title"`
	Frame        Rect   `json:"frame"`
	DisplayID    string `json:"displayId"`
	IsFullScreen bool   `json:"isFullScreen"`
	IsMinimized  bool   `json:"isMinimized"`
	IsMovable    bool   `json:"isMovable"`
	IsResizable  bool   `json:"isResizable"`
}

// ListedWindow is one entry of `all`: a window with its owner and stacking position.
type ListedWindow struct {
	Window
	PID     int    `json:"pid"`
	AppName string `json:"appName"`
	ZIndex  int    `json:"zIndex"`
}

// App is one entry of `apps`.
type App struct {
	PID      int    `json:"pid"`
	BundleID string `json:"bundleId"`
	Name     string `json:"name"`
}

// FrameResult is what `set-frame` and `set-fullscreen` report. Outcome is "exact", "partial" or "unapplied".
type FrameResult struct {
	Outcome string `json:"outcome"`
	Changed bool   `json:"changed"`
	Frame   Rect   `json:"frame"`
	Reason  string `json:"reason"`
}

// PlatformWindow converts a helper window to the port's record. The owner, its name and the stacking position
// come from the caller: `list` and `focused` do not print them, and `all` prints them on the listed window.
func (w Window) PlatformWindow(pid int, appName string, zIndex int) platform.WindowRecord {
	return platform.WindowRecord{
		Key:        w.WindowKey,
		PID:        pid,
		BundleID:   w.BundleID,
		AppName:    appName,
		Title:      w.Title,
		Frame:      w.Frame.Platform(),
		DisplayID:  w.DisplayID,
		FullScreen: w.IsFullScreen,
		Minimized:  w.IsMinimized,
		Movable:    w.IsMovable,
		Resizable:  w.IsResizable,
		ZIndex:     zIndex,
	}
}
