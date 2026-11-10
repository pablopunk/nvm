package supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// A UI state notification refreshes the root, cancelling its current search.
func TestSearchReplacementDoesNotRefreshItsOwnQuery(t *testing.T) {
	for _, hung := range []bool{false, true} {
		t.Run(map[bool]string{false: "response", true: "hard budget"}[hung], func(t *testing.T) {
			clock := fake.NewClock()
			worker := &searchWorker{started: make(chan *searchProcess, 3), holdRestarts: true}
			var mu sync.Mutex
			var active context.CancelFunc
			callbacks := make(chan struct{}, 16)
			r := New(Config{Clock: clock, Worker: worker, Budgets: map[string]Budget{protocol.MethodSearch: {Hard: 13 * time.Second}}, OnStateChange: func() {
				mu.Lock()
				cancel := active
				mu.Unlock()
				if cancel != nil {
					cancel()
				}
				callbacks <- struct{}{}
			}}, api.Identity{ID: "search"}, "", nil)
			t.Cleanup(r.Stop)
			r.Start()
			old := receiveSearch(t, worker.started)
			receiveSearch(t, callbacks)
			receiveSearch(t, callbacks)
			ctx, cancel := context.WithCancel(context.Background())
			oldDone := make(chan error, 1)
			go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); oldDone <- err }()
			readSearchRequest(t, old)
			clock.AwaitTimerDueIn(13 * time.Second)
			clock.Advance(time.Second)
			cancel()
			if err := receiveSearch(t, oldDone); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			receiveSearch(t, old.reaped)
			next, cancelNext := context.WithCancel(context.Background())
			defer cancelNext()
			mu.Lock()
			active = cancelNext
			mu.Unlock()
			result := make(chan callResult, 1)
			go func() {
				response, err := r.Call(next, protocol.MethodSearch, "new", nil)
				result <- callResult{response: response, err: err}
			}()
			fresh := receiveSearch(t, worker.started)
			fresh.lines <- []byte(`{"ok":true,"methods":["run","search"]}`)
			req := readSearchRequest(t, fresh)
			if hung {
				clock.AwaitTimerDueIn(13 * time.Second)
				clock.Advance(13 * time.Second)
				outcome := receiveSearch(t, result)
				receiveSearch(t, callbacks) // the actual failure must still notify the UI
				if got := r.Status(); got.State != StateFailed || got.Failures != 1 {
					t.Fatalf("failure not recorded: %+v; call=%v", got, outcome.err)
				}
			} else {
				replySearch(t, fresh, req, `[]`)
				outcome := receiveSearch(t, result)
				if outcome.err != nil || string(outcome.response.Data) != `[]` {
					t.Fatalf("replacement: %+v", outcome)
				}
			}
			mu.Lock()
			active = nil
			mu.Unlock()
		})
	}
}

func TestReplacementWithChangedMethodsNotifies(t *testing.T) {
	worker := &searchWorker{started: make(chan *searchProcess, 2), holdRestarts: true}
	callbacks := make(chan struct{}, 8)
	r := New(Config{Clock: fake.NewClock(), Worker: worker, OnStateChange: func() { callbacks <- struct{}{} }}, api.Identity{ID: "search"}, "", nil)
	t.Cleanup(r.Stop)
	r.Start()
	old := receiveSearch(t, worker.started)
	receiveSearch(t, callbacks)
	receiveSearch(t, callbacks)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); done <- err }()
	readSearchRequest(t, old)
	cancel()
	receiveSearch(t, done)
	receiveSearch(t, old.reaped)
	result := make(chan callResult, 1)
	go func() {
		response, err := r.Call(context.Background(), protocol.MethodSearch, "new", nil)
		result <- callResult{response: response, err: err}
	}()
	fresh := receiveSearch(t, worker.started)
	fresh.lines <- []byte(`{"ok":true,"methods":["search"]}`)
	receiveSearch(t, callbacks)
	req := readSearchRequest(t, fresh)
	replySearch(t, fresh, req, `[]`)
	if outcome := receiveSearch(t, result); outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if r.Provides(protocol.MethodRun) {
		t.Fatal("replacement retained removed method")
	}
}
