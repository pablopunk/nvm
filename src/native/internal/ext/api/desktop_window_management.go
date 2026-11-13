package api

import "time"

// FocusResult reports a focus request. Failures arrive with OK false and a palette-ready Message.
type FocusResult struct {
	OK      bool   `json:"ok"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// WindowManagement moves, resizes and focuses windows of other apps. Coordinates are global
// points with a top-left origin. Handles are opaque and valid only in this session.
type WindowManagement interface {
	// Apply lays out the focused window of the app that was frontmost before Nevermind opened.
	Apply(layout WindowLayout, options WindowManagementOptions) (ApplyResult, error)
	ApplyTo(handle WindowHandle, layout WindowLayout, options WindowManagementOptions) (ApplyResult, error)
	Displays() ([]DisplayInfo, error)
	Windows(query WindowQuery) ([]WindowInfo, error)
	// WaitForWindow polls Windows until a match exists or timeout passes (default 5 s, max 30 s).
	WaitForWindow(query WindowQuery, timeout time.Duration) (window WindowInfo, found bool, err error)
	Focus(handle WindowHandle) (FocusResult, error)
	// FocusNeighbor focuses the nearest window in a direction: left, right, up or down.
	FocusNeighbor(direction string, wrap bool) (FocusResult, error)
	// SwitchDesktop moves to the next or previous desktop (Space), as the Mission Control shortcut does. Direction is
	// next or previous. Platform: macOS only (see PlatformOperations); it needs Automation access to System Events.
	// An extension whose source calls it is not run on another OS.
	SwitchDesktop(direction string) (FocusResult, error)
}
