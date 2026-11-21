package supervisor

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCancelledSearchReplacementStartupBoundaries(t *testing.T) {
	for _, ending := range []string{"ready", "stop", "broken", "crashed"} {
		t.Run(ending, func(t *testing.T) {
			worker := &searchWorker{started: make(chan *searchProcess, 3), holdRestarts: true}
			r := New(Config{Clock: fake.NewClock(), Worker: worker}, api.Identity{ID: "search"}, "", nil)
			t.Cleanup(r.Stop)
			r.Start()
			old := receiveSearch(t, worker.started)
			waitCtx, stopWaiting := context.WithTimeout(context.Background(), waitTimeout)
			defer stopWaiting()
			if err := r.WaitFor(waitCtx, func(s Status) bool { return s.State == StateRunning }); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(waitCtx)
			oldDone := make(chan error, 1)
			go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); oldDone <- err }()
			readSearchRequest(t, old)
			cancel()
			if err := receiveSearch(t, oldDone); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			receiveSearch(t, old.reaped)
			result := make(chan callResult, 1)
			go func() {
				response, err := r.Call(waitCtx, protocol.MethodSearch, "new", nil)
				result <- callResult{response: response, err: err}
			}()
			fresh := receiveSearch(t, worker.started)
			if !r.Provides(protocol.MethodSearch) || !r.Provides(protocol.MethodRun) || r.Status().State != StateDormant {
				t.Fatal("declared methods disappeared during held replacement startup")
			}
			select {
			case request := <-fresh.writes:
				t.Fatalf("request dispatched before ready: %s", request)
			default:
			}
			switch ending {
			case "ready":
				fresh.lines <- []byte(`{"ok":true,"methods":["search","run"]}`)
				req := readSearchRequest(t, fresh)
				if req.Query != "new" {
					t.Fatalf("replacement query = %q", req.Query)
				}
				replySearch(t, fresh, req, `[{"id":"fresh"}]`)
				outcome := receiveSearch(t, result)
				if outcome.err != nil || string(outcome.response.Data) != `[{"id":"fresh"}]` {
					t.Fatalf("replacement: %+v", outcome)
				}
				return
			case "stop":
				r.Stop()
				// A late ready line must not adopt the stopped generation.
				fresh.lines <- []byte(`{"ok":true,"methods":["search","run"]}`)
			case "broken":
				fresh.lines <- []byte(`{"ok":false,"failure":{"code":"internal","message":"replacement failed to load"}}`)
			case "crashed":
				if err := fresh.Kill(); err != nil {
					t.Fatal(err)
				}
			}
			if outcome := receiveSearch(t, result); outcome.err == nil {
				t.Fatal("failed or stopped startup admitted request")
			}
			receiveSearch(t, fresh.reaped)
			if r.Provides(protocol.MethodSearch) {
				t.Fatal("stopped or failed startup kept advertising search")
			}
			want := map[string]State{"stop": StateIdle, "broken": StateBroken, "crashed": StateFailed}[ending]
			if got := r.Status(); got.State != want {
				t.Fatalf("startup terminal state = %+v, want %s", got, want)
			}
		})
	}
}

func TestLateResponseWhileSearchRetirementIsHeldCannotReachReplacement(t *testing.T) {
	worker := &searchWorker{started: make(chan *searchProcess, 2)}
	r := New(Config{Clock: fake.NewClock(), Worker: worker}, api.Identity{ID: "search"}, "", nil)
	t.Cleanup(r.Stop)
	r.Start()
	old := receiveSearch(t, worker.started)
	waitCtx, stopWaiting := context.WithTimeout(context.Background(), waitTimeout)
	defer stopWaiting()
	if err := r.WaitFor(waitCtx, func(s Status) bool { return s.State == StateRunning }); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	old.killEntered = make(chan struct{})
	old.killRelease = release
	ctx, cancel := context.WithCancel(waitCtx)
	done := make(chan error, 1)
	go func() { _, err := r.Call(ctx, protocol.MethodSearch, "old", nil); done <- err }()
	req := readSearchRequest(t, old)
	cancel()
	receiveSearch(t, old.killEntered)
	// Queue the response while the old child is detached but its kill has not completed.
	replySearch(t, old, req, `[{"id":"late"}]`)
	close(release)
	if err := receiveSearch(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	receiveSearch(t, old.reaped)
	result := make(chan callResult, 1)
	go func() {
		response, err := r.Call(waitCtx, protocol.MethodSearch, "new", nil)
		result <- callResult{response: response, err: err}
	}()
	fresh := receiveSearch(t, worker.started)
	req = readSearchRequest(t, fresh)
	replySearch(t, fresh, req, `[]`)
	outcome := receiveSearch(t, result)
	if outcome.err != nil || string(outcome.response.Data) != `[]` {
		t.Fatalf("late response contaminated replacement: %+v", outcome)
	}
}
