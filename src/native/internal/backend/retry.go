package backend

import (
	"math/rand/v2"
	"time"
)

// retryPolicy bounds the retries of idempotent GET calls. Only transient failures are retried: offline,
// timeout and server errors. Streams, POSTs and rate limits are never retried here.
type retryPolicy struct {
	// attempts counts every try, the first included.
	attempts int
	// base is the delay before the first retry; each later retry doubles it.
	base time.Duration
	// max caps the delay before jitter.
	max time.Duration
}

// defaultRetryPolicy is three attempts with a 250 ms base and a 4 s cap.
func defaultRetryPolicy() retryPolicy {
	return retryPolicy{attempts: 3, base: 250 * time.Millisecond, max: 4 * time.Second}
}

// delay returns the un-jittered wait before retry number retry (1 for the first retry).
func (p retryPolicy) delay(retry int) time.Duration {
	if p.base <= 0 || retry < 1 {
		return 0
	}
	wait := p.base
	for i := 1; i < retry; i++ {
		wait *= 2
		if p.max > 0 && wait >= p.max {
			return p.max
		}
	}
	if p.max > 0 && wait > p.max {
		return p.max
	}
	return wait
}

// defaultJitter returns a random duration in [d/2, d], so simultaneous clients spread their retries.
func defaultJitter(d time.Duration) time.Duration {
	half := d / 2
	if half <= 0 {
		return d
	}
	return half + time.Duration(rand.Int64N(int64(d-half)+1))
}

// isTransientKind reports whether a failure kind is worth retrying an idempotent call for.
func isTransientKind(kind Kind) bool {
	return kind == KindOffline || kind == KindTimeout || kind == KindServer
}
