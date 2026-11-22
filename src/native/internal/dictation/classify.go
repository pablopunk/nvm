package dictation

import (
	"context"
	"errors"
	"syscall"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// transientFailure reports whether trying again may succeed. Only a failure that is known to be permanent fails at
// once: a refusal of the account (sign-in, credits, the spend cap, an incompatible version, any other request the
// backend rejected), a refused microphone, a missing codec, a system without microphone capture, a recording that is
// gone, or a cancellation. Everything else is retried, including failures nobody classified.
func transientFailure(err error) bool {
	if err == nil {
		return false
	}
	if _, capped := auth.SpendCapNoticeFromError(err); capped {
		return false
	}
	var typed *backend.Error
	if errors.As(err, &typed) {
		return transientBackendKind(typed.Kind)
	}
	switch {
	case errors.Is(err, ErrCancelled), errors.Is(err, context.Canceled), errors.Is(err, ErrBusy),
		errors.Is(err, ErrNoRecordings), errors.Is(err, ErrNoTranscriber), errors.Is(err, ErrCodec),
		errors.Is(err, ErrDeviceUnavailable),
		errors.Is(err, platform.ErrDenied), unsupportedSystem(err),
		errors.Is(err, recordings.ErrNotFound), errors.Is(err, recordings.ErrInvalidID):
		return false
	}
	return true
}

// transientBackendKind reports whether a backend failure of this kind may pass: the backend could not be reached,
// timed out, failed on its own, asked to slow down, or answered something this client could not read.
func transientBackendKind(kind backend.Kind) bool {
	switch kind {
	case backend.KindUnauthorized, backend.KindInsufficientCredits, backend.KindIncompatible, backend.KindRejected,
		backend.KindCanceled:
		return false
	}
	return true
}

// unsupportedSystem reports whether err says this system cannot capture or encode audio at all. platform.ErrUnsupported
// is the standard errors.ErrUnsupported, and a raw OS error such as ENOTSUP, EOPNOTSUPP or ENOSYS also matches it
// through syscall.Errno's Is; those come from one failed call (a process that could not start, a socket or a pipe that
// refused an operation), and a later try usually works. Only the port's own sentinel, with no OS error code behind it,
// means the system has no such capability.
func unsupportedSystem(err error) bool {
	if !errors.Is(err, platform.ErrUnsupported) {
		return false
	}
	var errno syscall.Errno
	return !errors.As(err, &errno)
}
