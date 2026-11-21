package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

type observerContextKey struct{}

type heldRunObserver struct {
	begin chan context.Context
	end   chan RunOutcome
}

func (o heldRunObserver) BeginRun(ctx context.Context) (context.Context, func(RunOutcome)) {
	ctx = context.WithValue(ctx, observerContextKey{}, "observed")
	o.begin <- ctx
	return ctx, func(outcome RunOutcome) { o.end <- outcome }
}

func TestRunObserverTracksSchedulerExecution(t *testing.T) {
	clock := fake.NewClock()
	observer := heldRunObserver{begin: make(chan context.Context, 3), end: make(chan RunOutcome, 3)}
	reporterCalls := make(chan safego.Failure, 1)
	sched, err := New(Config{
		Clock: clock, RunObserver: observer, Log: logx.Default(), Heartbeat: parkedHeartbeat,
		Reporter: safego.ReporterFunc(func(failure safego.Failure) string { reporterCalls <- failure; return "" }),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sched.Close() })

	entered := make(chan Run, 3)
	release := make(chan struct{})
	if err := sched.Register(Definition{ID: "observed", Title: "Observed", Triggers: eventOn("run", 0), Run: func(ctx context.Context, run Run) error {
		if ctx.Value(observerContextKey{}) != "observed" {
			t.Error("observer context value was not propagated")
		}
		entered <- run
		<-release
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	sched.Emit("run", nil)
	first := receiveRunTest(t, entered)
	if ctx := receiveContextTest(t, observer.begin); ctx.Value(observerContextKey{}) != "observed" {
		t.Fatal("begin did not receive derived context")
	}
	assertNoOutcomeTest(t, observer.end)

	if err := sched.RunNow("observed"); err != nil {
		t.Fatal(err)
	}
	awaitQueuedRunTest(t, sched, "observed")
	assertNoOutcomeTest(t, observer.end)
	close(release)
	second := receiveRunTest(t, entered)
	if first.Reason != "event:run" || second.Reason != ReasonManual {
		t.Fatalf("runs = %+v, %+v", first, second)
	}
	if got := receiveOutcomeTest(t, observer.end); got != RunSucceeded {
		t.Fatalf("first outcome = %q", got)
	}
	if ctx := receiveContextTest(t, observer.begin); ctx.Value(observerContextKey{}) != "observed" {
		t.Fatal("queued begin lost observer context")
	}
	if got := receiveOutcomeTest(t, observer.end); got != RunSucceeded {
		t.Fatalf("second outcome = %q", got)
	}

	awaitHistoryTest(t, sched, "observed", 2)
}

func TestRunObserverClassifiesCancellation(t *testing.T) {
	h := newHarness(t)
	observer := &recordingObserver{begin: make(chan context.Context, 1), end: make(chan RunOutcome, 1)}
	h.sched.runObserver = observer
	entered := make(chan struct{})
	if err := h.sched.Register(Definition{ID: "cancel-observed", Title: "Cancel", Triggers: eventOn("cancel", 0), Run: func(ctx context.Context, _ Run) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	h.sched.Emit("cancel", nil)
	awaitSignalTest(t, entered, "job did not enter")
	ctx := receiveContextTest(t, observer.begin)
	if err := h.sched.Close(); err != nil {
		t.Fatal(err)
	}
	if got := receiveOutcomeTest(t, observer.end); got != RunCancelled {
		t.Fatalf("outcome = %q", got)
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("observed context error = %v", ctx.Err())
	}
}

type recordingObserver struct {
	begin chan context.Context
	end   chan RunOutcome
}

func (o *recordingObserver) BeginRun(ctx context.Context) (context.Context, func(RunOutcome)) {
	o.begin <- ctx
	return ctx, func(outcome RunOutcome) { o.end <- outcome }
}

func awaitSignalTest(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(watchdog):
		t.Fatal(message)
	}
}

func receiveRunTest(t *testing.T, ch <-chan Run) Run {
	t.Helper()
	select {
	case run := <-ch:
		return run
	case <-time.After(watchdog):
		t.Fatal("run did not execute")
		return Run{}
	}
}
func receiveContextTest(t *testing.T, ch <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-ch:
		return ctx
	case <-time.After(watchdog):
		t.Fatal("observer did not begin")
		return context.Background()
	}
}
func receiveOutcomeTest(t *testing.T, ch <-chan RunOutcome) RunOutcome {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(watchdog):
		t.Fatal("observer did not finish")
		return ""
	}
}
func assertNoOutcomeTest(t *testing.T, ch <-chan RunOutcome) {
	t.Helper()
	select {
	case result := <-ch:
		t.Fatalf("observer finished before work: %q", result)
	default:
	}
}

func awaitQueuedRunTest(t *testing.T, sched *Scheduler, id string) {
	t.Helper()
	changes, cancel := sched.Subscribe()
	defer cancel()
	deadline := time.After(watchdog)
	for {
		sched.mu.Lock()
		queued := sched.jobs[id].pending != nil
		sched.mu.Unlock()
		if queued {
			return
		}
		select {
		case <-changes:
		case <-deadline:
			t.Fatal("manual run did not queue")
		}
	}
}

func awaitHistoryTest(t *testing.T, sched *Scheduler, id string, count int) {
	t.Helper()
	changes, cancel := sched.Subscribe()
	defer cancel()
	deadline := time.After(watchdog)
	for {
		if snapshot, found := findSnapshot(sched.Snapshot(), id); found && len(snapshot.History) >= count {
			return
		}
		select {
		case <-changes:
		case <-deadline:
			t.Fatal("scheduler did not preserve both run history entries")
		}
	}
}
