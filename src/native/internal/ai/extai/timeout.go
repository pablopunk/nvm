package extai

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// withDeadline returns ctx ending with ErrTimedOut once d has passed on clock. The returned cancel ends the context
// without a timeout and releases the timer; it is safe to call more than once.
func withDeadline(ctx context.Context, clock platform.Clock, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		select {
		case <-clock.After(d):
			cancel(ErrTimedOut)
		case <-ctx.Done():
		}
	}()
	return ctx, func() { cancel(context.Canceled) }
}
