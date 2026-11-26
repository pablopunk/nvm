package platform

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrUICallAbandoned is the failure of a call whose caller gave up before the UI thread took it. It never ran.
var ErrUICallAbandoned = errors.New("the change was stopped before it started; nothing changed")

// Phases of a call handed to the UI thread. Exactly one side wins the move out of uiCallQueued: the UI thread starts
// the call, or the caller abandons it, and an abandoned call never runs.
const (
	uiCallQueued int32 = iota
	uiCallStarted
	uiCallAbandoned
)

// CallOnUI posts call to the UI thread through onUI and waits for it. When ctx ends first, a call that has not started is
// abandoned and never runs (ErrUICallAbandoned); one that started is waited for, so the caller reports what really
// happened. It must not be called from the UI thread, which would wait for itself.
func CallOnUI(ctx context.Context, onUI func(func()), call func() error) error {
	var phase atomic.Int32
	done := make(chan error, 1)
	onUI(func() {
		if !phase.CompareAndSwap(uiCallQueued, uiCallStarted) {
			return
		}
		done <- call()
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if phase.CompareAndSwap(uiCallQueued, uiCallAbandoned) {
			return ErrUICallAbandoned
		}
		return <-done
	}
}
