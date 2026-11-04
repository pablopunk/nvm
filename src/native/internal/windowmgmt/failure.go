package windowmgmt

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// FailureReason says why a window command did not change anything. Values match the Electron engine so
// logs and palette copy line up across the two implementations.
type FailureReason string

const (
	ReasonNoTarget         FailureReason = "no-target"
	ReasonFocusNotRestored FailureReason = "focus-not-restored"
	ReasonNoWindow         FailureReason = "no-window"
	ReasonTargetLost       FailureReason = "target-lost"
	ReasonFullScreen       FailureReason = "full-screen"
	ReasonMinimized        FailureReason = "minimized"
	ReasonDenied           FailureReason = "denied"
	ReasonUnapplied        FailureReason = "unapplied"
	ReasonUnsupported      FailureReason = "unsupported"
	ReasonInvalidSpec      FailureReason = "invalid-spec"
	ReasonNoHistory        FailureReason = "no-history"
	ReasonUnknownWindow    FailureReason = "unknown-window"
	ReasonNoNeighbor       FailureReason = "no-neighbor"
	ReasonSingleDisplay    FailureReason = "single-display"
)

// Palette copy for failures that have no geometry message. Strings match window-management-failures.ts.
const (
	messageNoTargetPalette     = "No target app was captured. Reopen Nevermind from the app you want to move, then try again."
	messageNoTargetShortcut    = "No frontmost app was available when the shortcut ran."
	messageFocusNotRestored    = "Could not return focus to the target app. No window was changed."
	messageFocusChangedPalette = "The target app changed. Reopen Nevermind from the window you want to move."
	messageFocusChangedKey     = "The frontmost app changed before the window was moved. No window was changed."
	messageSelfTarget          = "Nevermind is in front. Focus the window you want to move, then run the command again."
	messageNoWindow            = "The target app has no window to move."
	messageNoFocusedWindow     = "The target app has no focused window."
	messageTargetLost          = "The target window is no longer available."
	messageDenied              = "Window access is not available. Grant window access, then try again."
	messageUnsupported         = "Window management is not supported on this device."
	messageUnknownWindow       = "That window handle is no longer valid. List windows again."
	messageNoNeighbor          = "No window lies in that direction."
	messageHelperFailed        = "The window helper failed. Restart Nevermind and try again."
)

var defaultFailureMessages = map[FailureReason]string{
	ReasonNoTarget:         messageNoTargetPalette,
	ReasonFocusNotRestored: messageFocusNotRestored,
	ReasonNoWindow:         messageNoWindow,
	ReasonTargetLost:       messageTargetLost,
	ReasonFullScreen:       geometry.MessageFullScreen,
	ReasonMinimized:        geometry.MessageMinimized,
	ReasonDenied:           messageDenied,
	ReasonUnapplied:        geometry.MessageUnapplied,
	ReasonUnsupported:      messageUnsupported,
	ReasonInvalidSpec:      geometry.MessageInvalidSpec,
	ReasonNoHistory:        geometry.MessageNoHistory,
	ReasonUnknownWindow:    messageUnknownWindow,
	ReasonNoNeighbor:       messageNoNeighbor,
	ReasonSingleDisplay:    geometry.MessageSingleDisplay,
}

// Failure is a command that did not reach its window. Message is the palette copy.
type Failure struct {
	Reason  FailureReason
	Message string
}

// Error returns the palette copy, so a Failure can be returned as an error.
func (f *Failure) Error() string {
	return f.Message
}

func newFailure(reason FailureReason, message string) *Failure {
	if message == "" {
		message = defaultFailureMessages[reason]
	}
	return &Failure{Reason: reason, Message: message}
}

// failureOf maps an error from a window manager call to a failure. Platform codes keep their meaning;
// anything unexpected is reported as an unapplied change with the helper's failure copy.
func failureOf(err error) *Failure {
	var windowErr *platform.WindowError
	if !errors.As(err, &windowErr) {
		return newFailure(ReasonUnapplied, messageHelperFailed)
	}
	return newFailure(reasonOfWindowCode(windowErr.Code), windowErr.Message)
}

func reasonOfWindowCode(code platform.WindowErrorCode) FailureReason {
	switch code {
	case platform.WindowErrorDenied:
		return ReasonDenied
	case platform.WindowErrorNoWindow:
		return ReasonNoWindow
	case platform.WindowErrorTargetLost:
		return ReasonTargetLost
	case platform.WindowErrorUnsupported:
		return ReasonUnsupported
	}
	return ReasonUnapplied
}

// isTargetGone reports whether the failure means the window or its process is no longer there.
func isTargetGone(err error) bool {
	var failed *Failure
	return errors.As(err, &failed) && (failed.Reason == ReasonTargetLost || failed.Reason == ReasonNoWindow)
}
