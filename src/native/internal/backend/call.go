package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// Body size limits. A success body larger than maxResponseBytes is a bad response; an error body is only
// read for its envelope, so it is cut at maxErrorBytes without failing.
const (
	maxResponseBytes int64 = 4 << 20
	maxErrorBytes    int64 = 64 << 10
)

// callJSON sends spec and decodes a success body into out. An idempotent call retries transient failures
// with jittered backoff through the clock; every other call makes exactly one attempt.
func (c *Client) callJSON(ctx context.Context, spec requestSpec, out any) error {
	attempts := 1
	if spec.idempotent {
		attempts = c.retry.attempts
	}
	for attempt := 1; ; attempt++ {
		err := c.callJSONOnce(ctx, spec, out)
		if err == nil || attempt >= attempts || !isTransientKind(kindOf(err)) || ctx.Err() != nil {
			return err
		}
		if waitErr := c.waitBackoff(ctx, attempt); waitErr != nil {
			return waitErr
		}
	}
}

// callJSONOnce makes one attempt and decodes the response.
func (c *Client) callJSONOnce(ctx context.Context, spec requestSpec, out any) error {
	ex, err := c.send(ctx, spec)
	if err != nil {
		return err
	}
	defer ex.release()
	body, tooLarge, readErr := readCapped(ex.body, maxResponseBytes)
	if readErr != nil {
		return c.readFailure(ctx, ex, readErr)
	}
	if ex.status < 200 || ex.status > 299 {
		return errorFromResponse(ex.status, ex.retryAfter, truncate(body, maxErrorBytes))
	}
	if tooLarge {
		return &Error{Kind: KindBadResponse, Status: ex.status, Message: fmt.Sprintf("response is larger than %d bytes", maxResponseBytes)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &Error{Kind: KindBadResponse, Status: ex.status, Message: "response is not the expected JSON", Cause: err}
	}
	return nil
}

// readFailure classifies an error reading the body and feeds it to the offline tracker.
func (c *Client) readFailure(ctx context.Context, ex exchange, readErr error) *Error {
	classified := classifyTransport(readErr, ex.timedOut(), ctx)
	c.noteFailure(classified.Kind)
	return classified
}

// waitBackoff waits the jittered delay before retry number attempt. It returns a canceled or timed-out *Error
// when the caller's context ends first.
func (c *Client) waitBackoff(ctx context.Context, attempt int) error {
	return c.pause(ctx, c.jitter(c.retry.delay(attempt)))
}

// readCapped reads at most limit bytes. tooLarge reports that the body had more; the returned bytes are then
// the first limit bytes.
func readCapped(body io.Reader, limit int64) ([]byte, bool, error) {
	if body == nil {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return data, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

// truncate returns at most limit bytes of data.
func truncate(data []byte, limit int64) []byte {
	if int64(len(data)) > limit {
		return data[:limit]
	}
	return data
}

// kindOf returns the Kind of a backend error, or the empty Kind for anything else.
func kindOf(err error) Kind {
	if typed := asError(err); typed != nil {
		return typed.Kind
	}
	return ""
}
