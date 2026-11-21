package backend

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// StreamIdleTimeout bounds how long a stream may go without a single byte, from sending the request until its body
// ends. Keep-alive comments and reasoning count as bytes, so a model that thinks for a long time keeps its stream; a
// connection that went silent ends as a timeout instead of waiting forever.
const StreamIdleTimeout = 120 * time.Second

// progressWatch ends a request that stops making progress. When no response byte arrives for window, it cancels the
// request and closes the body, so a read waiting on a silent connection returns. Every byte read restarts the window.
type progressWatch struct {
	clock    platform.Clock
	window   time.Duration
	cancel   context.CancelFunc
	progress chan struct{}
	done     chan struct{}
	finish   sync.Once
	fired    atomic.Bool
	mu       sync.Mutex
	body     io.Closer
}

// watchProgress derives the request context of a call bounded by window. A non-positive window watches nothing.
func (c *Client) watchProgress(parent context.Context, window time.Duration) (context.Context, *progressWatch) {
	ctx, cancel := context.WithCancel(parent)
	watch := &progressWatch{clock: c.clock, window: window, cancel: cancel, progress: make(chan struct{}, 1), done: make(chan struct{})}
	if window > 0 {
		go watch.run()
	}
	return ctx, watch
}

// run restarts the window on every sign of progress and gives up when a whole window passes without one.
func (w *progressWatch) run() {
	for {
		select {
		case <-w.clock.After(w.window):
			w.giveUp()
			return
		case <-w.progress:
		case <-w.done:
			return
		}
	}
}

// giveUp cancels the request and closes the body that is being read, if it arrived.
func (w *progressWatch) giveUp() {
	w.mu.Lock()
	w.fired.Store(true)
	body := w.body
	w.mu.Unlock()
	w.cancel()
	if body != nil {
		_ = body.Close()
	}
}

// track wraps the response body so every read that returns bytes counts as progress. A body that arrives after the
// watch gave up is closed at once.
func (w *progressWatch) track(body io.ReadCloser) io.ReadCloser {
	w.mu.Lock()
	w.body = body
	gaveUp := w.fired.Load()
	w.mu.Unlock()
	if gaveUp {
		_ = body.Close()
	}
	return progressBody{ReadCloser: body, watch: w}
}

// noteProgress restarts the window without blocking the reader.
func (w *progressWatch) noteProgress() {
	select {
	case w.progress <- struct{}{}:
	default:
	}
}

// stop ends the watch and cancels the request context. It is safe to call more than once.
func (w *progressWatch) stop() {
	w.finish.Do(func() { close(w.done) })
	w.cancel()
}

// stalled reports whether the watch gave up on the request.
func (w *progressWatch) stalled() bool {
	return w.fired.Load()
}

// progressBody is a response body that reports each read with bytes to its watch.
type progressBody struct {
	io.ReadCloser
	watch *progressWatch
}

// Read reads from the body and counts the bytes as progress.
func (b progressBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.watch.noteProgress()
	}
	return n, err
}
