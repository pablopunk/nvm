package geometry

import (
	"errors"
	"fmt"
)

// User-facing copy, identical to the Electron source (window-management-failures.ts) except
// MessagePartialMinimumSize, which is new copy the source lacks (see the WP4.1a report).
const (
	MessageInvalidSpec         = "The window layout request is invalid."
	MessageNoHistory           = "There is no earlier position to restore for this window."
	MessageSingleDisplay       = "Only one display is connected."
	MessageUnapplied           = "The window did not accept the new frame."
	MessageNoDisplay           = "No display is available to place the window on."
	MessageCorrectionFailed    = "Part of the window stayed outside its display."
	MessageFullScreen          = "Exit full screen before moving this window."
	MessageMinimized           = "Restore the window before moving it."
	MessageTargetWindowChanged = "The focused window now belongs to a different app."
	MessagePartialMinimumSize  = "The window stayed larger than requested because the app enforces a minimum size."
	MessagePartialGeneric      = "The window only partly reached the requested frame."
	MessageNoFieldsExact       = "Choose at least one window value: x, y, width, or height."
)

var (
	ErrInvalidSpec   = copyError(MessageInvalidSpec)
	ErrNoHistory     = copyError(MessageNoHistory)
	ErrSingleDisplay = copyError(MessageSingleDisplay)
	ErrNoDisplay     = copyError(MessageNoDisplay)
	ErrNoWorkArea    = errors.New("the display work area must have positive width and height")
	ErrUnknownCycle  = errors.New("unknown cycle group")
)

// ServiceHandledError reports that a layout kind needs the engine (display moves, restore history,
// full screen) and has no frame arithmetic of its own. The text matches the Electron throw.
func ServiceHandledError(kind LayoutKind) error {
	return fmt.Errorf("%s is handled by the window management service", kind)
}

// ErrUnhandledKind reports a layout kind that has no frame arithmetic in Resolve.
var ErrUnhandledKind = errors.New("unhandled layout kind")

// copyError is a user-facing message that is also an error. Palette copy starts with a capital letter, which
// the error-string style check rejects in errors.New, so the text is carried as a string type instead.
type copyError string

func (e copyError) Error() string {
	return string(e)
}
