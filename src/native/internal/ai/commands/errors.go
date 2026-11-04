package commands

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// userError is a failure whose text is already the copy the user sees.
type userError string

func (e userError) Error() string { return string(e) }

// The failures a command names. Each carries the words the indicator shows.
const (
	errNoSelection       = userError("Could not read selected text. Select it and try again")
	ErrNoCorrection      = userError("AI returned no corrected text")
	ErrCorrectionRefused = userError("AI did not return a valid correction")
)

// signInCopy is the message of every command that needs an account.
const signInCopy = "Sign in to use AI commands."

// failureMessage is the copy the indicator shows for err.
func failureMessage(err error) string {
	var typed userError
	switch {
	case errors.As(err, &typed):
		return string(typed)
	case errors.Is(err, errNotSignedIn):
		return signInCopy
	case errors.Is(err, selection.ErrSourceGone), errors.Is(err, selection.ErrFormattedClipboard), errors.Is(err, selection.ErrKeyboardHeld):
		return err.Error()
	case errors.Is(err, platform.ErrConflict):
		return "Frontmost app changed. Select the text and try again"
	case errors.Is(err, platform.ErrDenied):
		return "Allow Nevermind to control the keyboard in System Settings to read selected text"
	case errors.Is(err, platform.ErrUnsupported):
		return "Selected text is not available on this system yet"
	}
	return backend.UserMessage(err)
}

// errNotSignedIn is the failure of a command started without a connected account.
var errNotSignedIn = errors.New("commands: not signed in")
