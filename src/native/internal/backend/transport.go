package backend

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
)

// exchange is one answered request: the status, the Retry-After header and the unread body. The caller must
// call release exactly once, after reading or closing the body.
type exchange struct {
	status     int
	retryAfter string
	body       io.ReadCloser
	timedOut   func() bool
	release    func()
}

// send performs one attempt of spec. It returns the response, or a classified *Error when no response
// arrived or the request could not be built. It updates the offline tracker and logs one line per attempt.
func (c *Client) send(ctx context.Context, spec requestSpec) (exchange, error) {
	started := c.clock.Now()
	reqCtx, stop, timedOut := c.bindTimeout(ctx, spec.timeout)
	req, err := c.buildRequest(reqCtx, spec)
	if err != nil {
		stop()
		return exchange{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		stop()
		classified := classifyTransport(err, timedOut(), ctx)
		c.noteFailure(classified.Kind)
		c.logAttempt(spec, 0, started, classified)
		return exchange{}, classified
	}
	if c.offline.reachable() {
		c.logger.Info("backend reachable again", logx.F{"origin": c.env.Kind})
	}
	c.logAttempt(spec, resp.StatusCode, started, nil)
	return exchange{
		status:     resp.StatusCode,
		retryAfter: resp.Header.Get("Retry-After"),
		body:       resp.Body,
		timedOut:   timedOut,
		release: func() {
			_ = resp.Body.Close()
			stop()
		},
	}, nil
}

// bindTimeout derives the request context. With a positive timeout it starts a clock ticker that cancels the
// context when the timeout passes, so tests advance the fake clock to trigger it. The returned stop function
// must be called once the request is finished; it stops the ticker and cancels the context.
func (c *Client) bindTimeout(parent context.Context, timeout time.Duration) (context.Context, func(), func() bool) {
	ctx, cancel := context.WithCancel(parent)
	if timeout <= 0 {
		return ctx, cancel, func() bool { return false }
	}
	ticker := c.clock.NewTicker(timeout)
	var fired atomic.Bool
	go func() {
		select {
		case <-ticker.C():
			fired.Store(true)
			cancel()
		case <-ctx.Done():
		}
	}()
	stop := func() {
		ticker.Stop()
		cancel()
	}
	return ctx, stop, fired.Load
}

// clockPause waits d on the clock, or returns a classified error when ctx ends first.
func (c *Client) clockPause(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return classifyTransport(ctx.Err(), false, ctx)
	case <-c.clock.After(d):
		return nil
	}
}

// noteFailure feeds a failed attempt to the offline tracker. Only network kinds count; a cancelled caller
// says nothing about the network.
func (c *Client) noteFailure(kind Kind) {
	if !isNetworkKind(kind) {
		return
	}
	if c.offline.unreachable() {
		c.logger.Warn("backend unreachable; requests continue and recovery is detected on the next response", logx.F{"kind": string(kind)})
	}
}

// logAttempt writes one debug line per attempt. It logs the method, the route without its query and the
// outcome. It never logs headers, so the token cannot appear here.
func (c *Client) logAttempt(spec requestSpec, status int, started time.Time, failure *Error) {
	fields := logx.F{
		"method":      spec.method,
		"path":        spec.path,
		"status":      status,
		"duration_ms": c.clock.Now().Sub(started).Milliseconds(),
	}
	if failure != nil {
		fields["kind"] = string(failure.Kind)
	}
	c.logger.Debug("backend request", fields)
}
