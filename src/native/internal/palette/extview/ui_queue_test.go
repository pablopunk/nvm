package extview

import (
	"sync"
	"testing"
	"time"
)

// uiQueue stands in for the UI thread in tests. Callbacks posted from goroutines wait here until the test settles
// them on its own goroutine, so no test state is touched from two goroutines at once.
type uiQueue struct {
	mu     sync.Mutex
	fns    []func()
	signal chan struct{}
}

func newUIQueue() *uiQueue {
	return &uiQueue{signal: make(chan struct{}, 64)}
}

func (q *uiQueue) post(fn func()) {
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// settle waits for one posted callback, then runs every callback queued so far. The watchdog only fails a test that
// waits for work that never comes; it is not a delay.
func (q *uiQueue) settle(t *testing.T) {
	t.Helper()
	select {
	case <-q.signal:
	case <-time.After(5 * time.Second):
		t.Fatal("no callback reached the UI thread")
	}
	q.run()
}

// run executes the queued callbacks in order on the calling goroutine.
func (q *uiQueue) run() {
	q.mu.Lock()
	fns := q.fns
	q.fns = nil
	q.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
