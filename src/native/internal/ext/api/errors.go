package api

import (
	"context"
	"errors"
)

// Sentinel errors a service or handler may return. The host maps them to ErrorCode values for
// the palette, so authors should prefer them over ad-hoc messages.
var (
	ErrNotAvailable  = errors.New("not available on this system")
	ErrPermission    = errors.New("permission denied by the operating system")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotFound      = errors.New("not found")
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrUnsupported is the failure of an operation that this operating system does not have (see
	// PlatformOperations), such as AppleScript off macOS. It is a kind of ErrNotAvailable, so code that checks for
	// that keeps working. The host never shows it to the user: an extension that needs the operation is not run on
	// such a system at all, and this is only what a call that still reaches the host gets.
	ErrUnsupported error = unsupportedError{}
)

// unsupportedError is ErrUnsupported: its own error, which also answers to ErrNotAvailable.
type unsupportedError struct{}

func (unsupportedError) Error() string { return "not supported on this operating system" }

func (unsupportedError) Is(target error) bool { return target == ErrNotAvailable }

// ErrorCode classifies a failure for the palette and for diagnostics.
type ErrorCode string

const (
	CodeInternal    ErrorCode = "internal"
	CodeUnavailable ErrorCode = "unavailable"
	// CodeUnsupported is ErrUnsupported on the wire: the operation does not exist on this operating system.
	CodeUnsupported ErrorCode = "unsupported"
	CodePermission  ErrorCode = "permission"
	CodeInvalid     ErrorCode = "invalid-input"
	CodeNotFound    ErrorCode = "not-found"
	CodeQuota       ErrorCode = "quota"
	CodeTimeout     ErrorCode = "timeout"
	CodeCancelled   ErrorCode = "cancelled"
)

// Failure is the serializable form of an error. Message is palette-safe text and must not contain
// file contents, clipboard text or secrets. Reference is an id that diagnostics can look up.
type Failure struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Reference string    `json:"reference,omitempty"`
}

// FailureOf maps an error to a Failure. Unknown errors become CodeInternal with their text, which
// the host still redacts before display.
func FailureOf(err error) Failure {
	switch {
	case err == nil:
		return Failure{}
	case errors.Is(err, context.Canceled):
		return Failure{Code: CodeCancelled, Message: "the operation was cancelled"}
	case errors.Is(err, context.DeadlineExceeded):
		return Failure{Code: CodeTimeout, Message: "the operation timed out"}
	case errors.Is(err, ErrUnsupported):
		return Failure{Code: CodeUnsupported, Message: err.Error()}
	case errors.Is(err, ErrNotAvailable):
		return Failure{Code: CodeUnavailable, Message: err.Error()}
	case errors.Is(err, ErrPermission):
		return Failure{Code: CodePermission, Message: err.Error()}
	case errors.Is(err, ErrInvalidInput):
		return Failure{Code: CodeInvalid, Message: err.Error()}
	case errors.Is(err, ErrNotFound):
		return Failure{Code: CodeNotFound, Message: err.Error()}
	case errors.Is(err, ErrQuotaExceeded):
		return Failure{Code: CodeQuota, Message: err.Error()}
	}
	return Failure{Code: CodeInternal, Message: err.Error()}
}
