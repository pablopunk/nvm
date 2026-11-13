package supervisor

import (
	"context"
	"errors"
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
			var active context.CancelFunc
			r := New(Config{Clock: clock, Worker: worker, Budgets: map[string]Budget{protocol.MethodSearch: {Hard: 13 * time.Second}}, OnStateChange: func() {
				cancel := active
				if cancel != nil {
					cancel()
				}
			}}, api.Identity{ID: "search"}, "", nil)
			t.Cleanup(r.Stop)
			drain := queuedStateChanges(r, t)
			r.Start()
			old := receiveSearch(t, worker.started)
			if err := r.WaitFor(context.Background(), func(s Status) bool { return s.State == StateRunning }); err != nil {
				t.Fatal(err)
			}
			drain(2)
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
			drain(0)
			active = cancelNext
			result := make(chan callResult, 1)
			go func() {
				response, err := r.Call(next, protocol.MethodSearch, "new", nil)
				result <- callResult{response: response, err: err}
			}()
			fresh := receiveSearch(t, worker.started)
			drain(0)
			fresh.lines <- []byte(`{"ok":true,"methods":["run","search"]}`)
			req := readSearchRequest(t, fresh)
			drain(0)
			if hung {
				clock.AwaitTimerDueIn(13 * time.Second)
				clock.Advance(13 * time.Second)
				outcome := receiveSearch(t, result)
				drain(1) // only the actual failure schedules a UI notification
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
			if !hung {
				drain(0)
			}
			active = nil
		})
	}
}

func TestReplacementWithChangedMethodsNotifies(t *testing.T) {
	worker := &searchWorker{started: make(chan *searchProcess, 2), holdRestarts: true}
	r := New(Config{Clock: fake.NewClock(), Worker: worker, OnStateChange: func() {}}, api.Identity{ID: "search"}, "", nil)
	t.Cleanup(r.Stop)
	drain := queuedStateChanges(r, t)
	r.Start()
	old := receiveSearch(t, worker.started)
	if err := r.WaitFor(context.Background(), func(s Status) bool { return s.State == StateRunning }); err != nil {
		t.Fatal(err)
	}
	drain(2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); done <- err }()
	readSearchRequest(t, old)
	cancel()
	receiveSearch(t, done)
	receiveSearch(t, old.reaped)
	drain(0)
	result := make(chan callResult, 1)
	go func() {
		response, err := r.Call(context.Background(), protocol.MethodSearch, "new", nil)
		result <- callResult{response: response, err: err}
	}()
	fresh := receiveSearch(t, worker.started)
	fresh.lines <- []byte(`{"ok":true,"methods":["search"]}`)
	req := readSearchRequest(t, fresh)
	drain(1)
	replySearch(t, fresh, req, `[]`)
	if outcome := receiveSearch(t, result); outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if r.Provides(protocol.MethodRun) {
		t.Fatal("replacement retained removed method")
	}
}

// Scheduling runs under r.mu. Each drain therefore observes every notification queued by the
// preceding synchronized transition, regardless of when a production goroutine would execute it.
func queuedStateChanges(r *Runtime, t *testing.T) func(int) {
	t.Helper()
	var pending []func()
	r.dispatchChange = func(fn func()) { pending = append(pending, fn) }
	return func(want int) {
		t.Helper()
		r.mu.Lock()
		callbacks := pending
		pending = nil
		r.mu.Unlock()
		if len(callbacks) != want {
			t.Fatalf("scheduled state callbacks = %d, want %d", len(callbacks), want)
		}
		for _, fn := range callbacks {
			fn()
		}
	}
}
