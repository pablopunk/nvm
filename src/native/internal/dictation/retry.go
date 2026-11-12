package dictation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// RetryPolicy bounds the automatic retries of a segment whose transcription failed for a transient reason. Attempts
// counts every try, the first included; the wait before retry n is Base doubled n-1 times, capped at Max. Sleep waits
// for that long and returns early with the context's error; nil uses a timer. The zero value is the default policy.
type RetryPolicy struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
	Sleep    func(ctx context.Context, wait time.Duration) error
}

const (
	defaultRetryAttempts = 3
	defaultRetryBase     = time.Second
	defaultRetryMax      = 4 * time.Second
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

// transientFailure reports whether trying the same audio again may succeed: the backend could not be reached, timed
// out, or failed on its own. Refusals (sign-in, credits, rate limit, version, rejection) and a cancelled call are
// permanent: another try changes nothing, or is not wanted.
func transientFailure(err error) bool {
	var typed *backend.Error
	if !errors.As(err, &typed) {
		return false
	}
	switch typed.Kind {
	case backend.KindOffline, backend.KindTimeout, backend.KindServer:
		return true
	}
	return false
}

// transcribeSegment sends one segment, retrying a transient failure under policy. A server failure may have settled its
// operation ID on the backend, so its retries use a fresh one; a lost connection or timeout may have reached the
// backend, so its retries repeat the ID and the backend answers the same request once.
func transcribeSegment(ctx context.Context, transcriber Transcriber, policy RetryPolicy, segment Segment) (string, error) {
	for attempt := 1; ; attempt++ {
		text, err := transcriber.Transcribe(ctx, segment)
		if err == nil || attempt >= policy.attempts() || ctx.Err() != nil || !transientFailure(err) {
			return text, err
		}
		if sleepErr := policy.sleep(ctx, policy.wait(attempt)); sleepErr != nil {
			return "", err
		}
		var typed *backend.Error
		if errors.As(err, &typed) && typed.Kind == backend.KindServer {
			segment.OperationID = fmt.Sprintf("%s-r%d", segment.OperationID, attempt)
			segment.Prepared = false
		}
	}
}
