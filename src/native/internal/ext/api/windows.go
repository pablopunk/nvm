package api

import (
	"errors"
	"fmt"
)

// Rect is a frame in global points with a top-left origin, or a fraction of a work area when the
// context says so.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// WindowOptions configures an independent window. Pointer fields keep the default when nil,
// because Go cannot tell "false" from "unset" for fields whose default is true.
type WindowOptions struct {
	// ID is local to the extension. It defaults to the view id, then a deterministic hash.
	ID string `json:"id,omitempty"`
	// RestoreKey is handed back to RestoreWindow after relaunch. It does not identify the live window.
	RestoreKey string   `json:"restoreKey,omitempty"`
	Title      string   `json:"title,omitempty"`
	TitleBar   string   `json:"titleBar,omitempty"` // default or hidden
	Chrome     string   `json:"chrome,omitempty"`   // default or none
	Width      int      `json:"width,omitempty"`    // clamped to 320..1600
	Height     int      `json:"height,omitempty"`   // clamped to 240..1200
	Size       ViewSize `json:"size,omitempty"`
	// AlwaysOnTop defaults to true.
	AlwaysOnTop *bool `json:"alwaysOnTop,omitempty"`
	// VisibleOnAllSpaces defaults to false.
	VisibleOnAllSpaces bool `json:"visibleOnAllSpaces,omitempty"`
	HideOnBlur         bool `json:"hideOnBlur,omitempty"`
	// Focusable defaults to true.
	Focusable         *bool  `json:"focusable,omitempty"`
	ShowInactive      bool   `json:"showInactive,omitempty"`
	IgnoreMouseEvents bool   `json:"ignoreMouseEvents,omitempty"`
	Position          string `json:"position,omitempty"` // center, top-center, bottom-center or menu-bar
	// Persistent retains the window for relaunch. It needs RestoreKey to be restored.
	Persistent     bool `json:"persistent,omitempty"`
	RemembersFrame bool `json:"remembersFrame,omitempty"`
}

// Validate checks the enumerated string fields.
func (o WindowOptions) Validate() error {
	if o.TitleBar != "" && o.TitleBar != "default" && o.TitleBar != "hidden" {
		return fmt.Errorf("window titleBar %q must be default or hidden", o.TitleBar)
	}
	if o.Chrome != "" && o.Chrome != "default" && o.Chrome != "none" {
		return fmt.Errorf("window chrome %q must be default or none", o.Chrome)
	}
	if o.Size != "" && !oneOf(o.Size, SizeDefault, SizeLarge) {
		return fmt.Errorf("window size %q must be default or large", o.Size)
	}
	return nil
}

// WindowRef names an existing owner-local window by ID, or describes a view to show in it.
type WindowRef struct {
	ID      string         `json:"id,omitempty"`
	View    *View          `json:"view,omitempty"`
	Options *WindowOptions `json:"options,omitempty"`
}

// WindowHandle is an opaque, short-lived reference returned by window listing. Never construct one.
type WindowHandle string

// WindowInfo describes a window of another app.
type WindowInfo struct {
	Handle     WindowHandle `json:"handle"`
	AppID      string       `json:"appId"`
	AppName    string       `json:"appName"`
	Title      string       `json:"title"`
	Frame      Rect         `json:"frame"`
	DisplayID  string       `json:"displayId"`
	FullScreen bool         `json:"fullScreen,omitempty"`
	Minimized  bool         `json:"minimized,omitempty"`
}

// DisplayInfo describes one display. WorkArea excludes the menu bar and Dock.
type DisplayInfo struct {
	ID       string `json:"id"`
	Primary  bool   `json:"primary,omitempty"`
	Frame    Rect   `json:"frame"`
	WorkArea Rect   `json:"workArea"`
}

// WindowQuery narrows window listing. All set fields must match.
type WindowQuery struct {
	AppID   string `json:"appId,omitempty"`
	AppName string `json:"appName,omitempty"`
	Title   string `json:"title,omitempty"`
}

// ApplyResult reports the outcome of a layout. Failures never panic or return a Go error for
// runtime conditions; they arrive with OK false and a palette-ready Message.
type ApplyResult struct {
	OK bool `json:"ok"`
	// Outcome is "exact" or "partial" (the app constrained the frame, for example a minimum size).
	Outcome string `json:"outcome,omitempty"`
	// Changed is false when the window already had the requested frame.
	Changed bool   `json:"changed,omitempty"`
	Frame   Rect   `json:"frame,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// LayoutKind selects how WindowLayout is applied.
type LayoutKind string

const (
	LayoutFraction         LayoutKind = "fraction"
	LayoutCenter           LayoutKind = "center"
	LayoutMaximizeWidth    LayoutKind = "maximize-width"
	LayoutMaximizeHeight   LayoutKind = "maximize-height"
	LayoutScaledCenter     LayoutKind = "scaled-center"
	LayoutEdge             LayoutKind = "edge"
	LayoutResizeStep       LayoutKind = "resize-step"
	LayoutDisplay          LayoutKind = "display"
	LayoutRestore          LayoutKind = "restore"
	LayoutToggleFullScreen LayoutKind = "toggle-full-screen"
	LayoutExact            LayoutKind = "exact"
)

// Dimension is one exact coordinate or size, in points or as a percent of the work area.
type Dimension struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"` // points or percent
}

// WindowLayout is a layout command. Kind selects which other fields are read, as documented on
// each LayoutKind. Validate checks the ranges.
type WindowLayout struct {
	Kind LayoutKind `json:"kind"`
	// Fraction is the frame for LayoutFraction; each value is 0..1 of the work area.
	Fraction *Rect `json:"fraction,omitempty"`
	// CycleGroup is left, right, top or bottom for repeatable half commands.
	CycleGroup string `json:"cycleGroup,omitempty"`
	// WidthFraction and HeightFraction, with MaxWidth and MaxHeight in points, for scaled-center.
	WidthFraction  float64 `json:"widthFraction,omitempty"`
	HeightFraction float64 `json:"heightFraction,omitempty"`
	MaxWidth       float64 `json:"maxWidth,omitempty"`
	MaxHeight      float64 `json:"maxHeight,omitempty"`
	// Edge is left, right, top or bottom for LayoutEdge.
	Edge string `json:"edge,omitempty"`
	// Direction is larger or smaller (resize-step), or next or previous (display).
	Direction string `json:"direction,omitempty"`
	// Step is a fraction of the work area, 0.01..0.25, for resize-step.
	Step float64 `json:"step,omitempty"`
	// Exact fields for LayoutExact. Omitted fields keep the current value.
	Exact *ExactFrame `json:"exact,omitempty"`
}

// ExactFrame places and sizes a window exactly.
type ExactFrame struct {
	X      *Dimension `json:"x,omitempty"`
	Y      *Dimension `json:"y,omitempty"`
	Width  *Dimension `json:"width,omitempty"`
	Height *Dimension `json:"height,omitempty"`
}

// WindowManagementOptions tunes a layout. Pointer fields keep their defaults when nil.
type WindowManagementOptions struct {
	// OuterGap is whole points (0..40) per edge; default 8 on every edge.
	OuterGap *Edges `json:"outerGap,omitempty"`
	// InnerGap is whole points (0..40) between adjacent tiles; default 4.
	InnerGap *int `json:"innerGap,omitempty"`
	// ApplyGapToMaximize defaults to true.
	ApplyGapToMaximize *bool `json:"applyGapToMaximize,omitempty"`
	// Cycle defaults to true.
	Cycle *bool `json:"cycle,omitempty"`
	// CycleOrder is half-two-thirds-third (default) or half-third-two-thirds.
	CycleOrder string `json:"cycleOrder,omitempty"`
	// DisplayOrder is left-to-right (default) or top-to-bottom.
	DisplayOrder string `json:"displayOrder,omitempty"`
	// DisplayMoveSizing is proportional (default) or keep-size.
	DisplayMoveSizing string `json:"displayMoveSizing,omitempty"`
}

// Edges is a per-edge measure in points.
type Edges struct {
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
}

// Validate checks the layout kind and the values that kind requires. The host applies the same
// rules and returns "invalid-spec" when they fail at runtime.
func (l WindowLayout) Validate() error {
	switch l.Kind {
	case LayoutFraction:
		if l.Fraction == nil {
			return errors.New("fraction layout needs a fraction rect")
		}
		if l.Fraction.Width <= 0 || l.Fraction.Height <= 0 || l.Fraction.X < 0 || l.Fraction.Y < 0 ||
			l.Fraction.X+l.Fraction.Width > 1 || l.Fraction.Y+l.Fraction.Height > 1 {
			return errors.New("fraction must stay inside the work area")
		}
	case LayoutEdge:
		if !oneOf(l.Edge, "left", "right", "top", "bottom") {
			return fmt.Errorf("edge %q is not left, right, top or bottom", l.Edge)
		}
	case LayoutResizeStep:
		if !oneOf(l.Direction, "larger", "smaller") || l.Step < 0.01 || l.Step > 0.25 {
			return errors.New("resize-step needs larger or smaller and a step from 0.01 to 0.25")
		}
	case LayoutDisplay:
		if !oneOf(l.Direction, "next", "previous") {
			return fmt.Errorf("display direction %q must be next or previous", l.Direction)
		}
	case LayoutExact:
		if l.Exact == nil {
			return errors.New("exact layout needs an exact frame")
		}
	case LayoutCenter, LayoutMaximizeWidth, LayoutMaximizeHeight, LayoutScaledCenter,
		LayoutRestore, LayoutToggleFullScreen:
		return nil
	default:
		return fmt.Errorf("unknown layout kind %q", l.Kind)
	}
	return nil
}
