package extai

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Time limits of a model call. A reply that takes longer is stopped and reported as a timeout.
const (
	CallTimeout   = 2 * time.Minute
	StreamTimeout = 5 * time.Minute
)

// ErrTimedOut is the failure of a call that ran past its time limit.
var ErrTimedOut = errors.New("extai: the model call took too long and was stopped")

// InputError is a call the host refuses because of what the extension sent: a bad tier, an unreadable attachment
// or an image that is too large. Its text is the message the extension sees, in the words 1.x uses.
type InputError string

func (e InputError) Error() string { return string(e) }

// inputErrorf formats an InputError.
func inputErrorf(format string, args ...any) error {
	return InputError(fmt.Sprintf(format, args...))
}

// timedOut reports whether ctx ended because its time limit passed.
func timedOut(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), ErrTimedOut)
}

// callError returns ErrTimedOut when the call's time limit ended it, and err otherwise.
func callError(ctx context.Context, err error) error {
	if err != nil && timedOut(ctx) {
		return ErrTimedOut
	}
	return err
}
