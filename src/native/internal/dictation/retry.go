package dictation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// RetryPolicy bounds the automatic retries of every dictation step that can fail for a passing reason: opening the
// microphone, reading the device list for the readiness wait, the readiness wait itself, reopening a microphone that
// stopped mid-recording, and each segment's transcription. Attempts counts every try, the first included; the wait
// before retry n is Base doubled n-1 times, capped at Max. Sleep waits for that long and returns early with the
// context's error; nil uses a timer. The zero value is the default policy: six tries over about thirty seconds.
type RetryPolicy struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
	Sleep    func(ctx context.Context, wait time.Duration) error
}

// The default policy waits 1, 2, 4, 8 and 15 seconds between six tries, 30 seconds in all, before the user sees an error.
const (
	defaultRetryAttempts = 6
	defaultRetryBase     = time.Second
	defaultRetryMax      = 15 * time.Second
)

func (p RetryPolicy) attempts() int {
	if p.Attempts <= 0 {
		return defaultRetryAttempts
	}
	return p.Attempts
}

// wait is the pause before retry number retry (1 for the first retry).
func (p RetryPolicy) wait(retry int) time.Duration {
	base, limit := p.Base, p.Max
	if base <= 0 {
		base = defaultRetryBase
	}
	if limit <= 0 {
		limit = defaultRetryMax
	}
	wait := base
	for i := 1; i < retry && wait < limit; i++ {
		wait *= 2
	}
	return min(wait, limit)
}

func (p RetryPolicy) sleep(ctx context.Context, wait time.Duration) error {
	if p.Sleep != nil {
		return p.Sleep(ctx, wait)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// do runs try until it succeeds, fails for a permanent reason, spends the attempts, or ctx ends, and returns the last
// error. onRetry, when set, hears each transient failure that is about to be retried.
func (p RetryPolicy) do(ctx context.Context, try func(attempt int) error, onRetry func(attempt int, err error)) error {
	for attempt := 1; ; attempt++ {
		err := try(attempt)
		if err == nil || attempt >= p.attempts() || ctx.Err() != nil || !transientFailure(err) {
			return err
		}
		if onRetry != nil {
			onRetry(attempt, err)
		}
		if p.sleep(ctx, p.wait(attempt)) != nil {
			return err
		}
	}
}

// transcribeSegment sends one segment, retrying a transient failure under policy. A server failure may have settled its
// operation ID on the backend, so its retries use a fresh one; a lost connection or timeout may have reached the
// backend, so its retries repeat the ID and the backend answers the same request once.
func transcribeSegment(ctx context.Context, transcriber Transcriber, policy RetryPolicy, segment Segment, onRetry func(int, error)) (string, error) {
	var text string
	err := policy.do(ctx, func(attempt int) error {
		var err error
		text, err = transcriber.Transcribe(ctx, segment)
		var typed *backend.Error
		if err != nil && errors.As(err, &typed) && typed.Kind == backend.KindServer {
			segment.OperationID = fmt.Sprintf("%s-r%d", segment.OperationID, attempt)
			segment.Prepared = false
		}
		return err
	}, onRetry)
	if err != nil {
		return "", err
	}
	return text, nil
}
