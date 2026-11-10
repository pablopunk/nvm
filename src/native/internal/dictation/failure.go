package dictation

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// SavedRecordingNotice follows a failure whose recording was kept, so the user knows where to retry it.
const SavedRecordingNotice = "Your recording was saved. Open Dictation History to retry."

// FailureMessage says what went wrong in a dictation, in words the user can act on. It is the single copy source
// for dictation failures, so the indicator, the toast and the account notice agree.
func FailureMessage(err error) string {
	var saved *SavedError
	if errors.As(err, &saved) {
		return FailureMessage(saved.Err) + " " + SavedRecordingNotice
	}
	if errors.Is(err, ErrCancelled) {
		return ""
	}
	if errors.Is(err, ErrBusy) {
		return "Dictation is already running."
	}
	if errors.Is(err, ErrCodec) {
		return "Dictation needs the Opus codec library (libopus), which is not installed on this system."
	}
	if notice, ok := auth.SpendCapNoticeFromError(err); ok {
		return notice.Message
	}
	if errors.Is(err, ErrNotReady) {
		return "The microphone did not start picking up sound. Check that it is connected, then try again."
	}
	if message, ok := microphoneFailureMessage(err); ok {
		return message
	}
	return backendFailureMessage(err)
}

func microphoneFailureMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, platform.ErrNotFound):
		return "No microphone was found. Connect one, or choose another in Dictation settings.", true
	case errors.Is(err, platform.ErrDenied):
		return "Microphone access is denied. Enable it in System Settings → Privacy & Security → Microphone.", true
	case errors.Is(err, platform.ErrUnsupported):
		return "Dictation is not available on this system yet.", true
	}
	return "", false
}

func backendFailureMessage(err error) string {
	var typed *backend.Error
	if !errors.As(err, &typed) {
		return "Dictation failed. Try again."
	}
	switch typed.Kind {
	case backend.KindUnauthorized:
		return "Sign in to use API dictation."
	case backend.KindInsufficientCredits:
		return "Not enough credits for API dictation. Add credits or try again later."
	case backend.KindRateLimited:
		return "Too many dictations right now. Wait a moment and try again."
	case backend.KindTimeout:
		return "Cloud transcription timed out. Try a shorter recording."
	case backend.KindOffline:
		return "Cloud transcription is unavailable. Check your connection and try again."
	case backend.KindIncompatible:
		return "This version of Nevermind is too old for API dictation. Update to keep dictating."
	case backend.KindServer:
		return "API dictation is unavailable right now. Try again in a moment."
	}
	if typed.Message != "" {
		return typed.Message
	}
	return "Dictation failed. Try again."
}
