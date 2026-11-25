package services

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// maxFailureMessage bounds the text a failure carries back to the child.
const maxFailureMessage = 512

// serviceError is a failure with its code already chosen. Its cause keeps errors.Is working for the caller.
type serviceError struct {
	failure api.Failure
	cause   error
}

func (e serviceError) Error() string {
	return e.failure.Message
}

func (e serviceError) Unwrap() error {
	return e.cause
}

func unavailable(format string, args ...any) error {
	return serviceError{failure: api.Failure{Code: api.CodeUnavailable, Message: fmt.Sprintf(format, args...)}, cause: api.ErrNotAvailable}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrInvalidInput)...)
}

// failureOf turns a service error into the wire failure, clipping the message to a palette-safe length.
func failureOf(err error) api.Failure {
	var typed serviceError
	failure := api.FailureOf(err)
	if errors.As(err, &typed) {
		failure = typed.failure
	}
	if len(failure.Message) > maxFailureMessage {
		failure.Message = failure.Message[:maxFailureMessage] + "..."
	}
	return failure
}
