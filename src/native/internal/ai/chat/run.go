package chat

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// run is one turn in flight. Stop may come from the UI thread while the turn's goroutine waits on the network,
// so the stream it holds is guarded.
type run struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	stream  *backend.Stream
	stopped bool
}

// newRun starts a turn's context. The caller stores the run and cancels it when the turn settles.
func newRun() *run {
	ctx, cancel := context.WithCancel(context.Background())
	return &run{ctx: ctx, cancel: cancel}
}

// stop cancels the turn and closes its stream, which unblocks a read waiting on the network.
func (r *run) stop() {
	r.mu.Lock()
	r.stopped = true
	stream := r.stream
	r.mu.Unlock()
	r.cancel()
	if stream != nil {
		_ = stream.Close()
	}
}

// attach records the open stream. It reports false when the turn was stopped first; the caller then closes it.
func (r *run) attach(stream *backend.Stream) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return false
	}
	r.stream = stream
	return true
}
