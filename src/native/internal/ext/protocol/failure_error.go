package protocol

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// FailureError is a failure that crossed the process boundary, as an error again. Its text is the message the other
// side wrote, which api.FailureOf already took from the whole wrapped error, so it is never wrapped with the sentinel's
// text a second time. errors.Is still finds the sentinel of its code, and api.FailureOf reads the same failure back.
type FailureError struct {
	Failure api.Failure
}

// ErrorOf is the error of a failure, or nil when there is none.
func ErrorOf(failure *api.Failure) error {
	if failure == nil {
		return nil
	}
	return FailureError{Failure: *failure}
}

func (e FailureError) Error() string {
	if e.Failure.Message != "" {
		return e.Failure.Message
	}
	if sentinel := sentinelOfCode(e.Failure.Code); sentinel != nil {
		return sentinel.Error()
	}
	return string(e.Failure.Code)
}

// Is matches the sentinel of the failure's code and what that sentinel matches, such as ErrNotAvailable for
// ErrUnsupported.
func (e FailureError) Is(target error) bool {
	sentinel := sentinelOfCode(e.Failure.Code)
	return sentinel != nil && (sentinel == target || errors.Is(sentinel, target))
}

func sentinelOfCode(code api.ErrorCode) error {
	return failureSentinels[code]
}

var failureSentinels = map[api.ErrorCode]error{
	api.CodeCancelled: context.Canceled, api.CodeTimeout: context.DeadlineExceeded, api.CodeUnsupported: api.ErrUnsupported,
	api.CodeUnavailable: api.ErrNotAvailable, api.CodePermission: api.ErrPermission, api.CodeInvalid: api.ErrInvalidInput,
	api.CodeNotFound: api.ErrNotFound, api.CodeQuota: api.ErrQuotaExceeded,
}
