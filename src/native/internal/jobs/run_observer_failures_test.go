package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

func TestRunObserverReportsBudgetTimeoutAndPreservesFailure(t *testing.T) {
	clock := fake.NewClock()
	observer := &recordingObserver{begin: make(chan context.Context, 1), end: make(chan RunOutcome, 1)}
	reporterCalls := make(chan safego.Failure, 1)
	sched, err := New(Config{
		Clock: clock, RunObserver: observer, Log: logx.Default(), Heartbeat: parkedHeartbeat,
		Reporter: safego.ReporterFunc(func(failure safego.Failure) string {
			reporterCalls <- failure
			return ""
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sched.Close() })

	entered := make(chan struct{})
	if err := sched.Register(Definition{
		ID: "budget-observed", Title: "Budget observed", Triggers: eventOn("budget", 0),
		Budget: Budget{Timeout: 3 * time.Second},
		Run: func(ctx context.Context, _ Run) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		},
	}); err != nil {
		t.Fatal(err)
	}
	sched.Emit("budget", nil)
	awaitSignalTest(t, entered, "run did not enter")
	_ = receiveContextTest(t, observer.begin)
	clock.AwaitTimerDueIn(3 * time.Second)
	clock.Advance(3 * time.Second)
	if got := receiveOutcomeTest(t, observer.end); got != RunTimedOut {
		t.Fatalf("observer outcome = %q, want %q", got, RunTimedOut)
	}
	assertNoOutcomeTest(t, observer.end)
	var failure safego.Failure
	select {
	case failure = <-reporterCalls:
	case <-time.After(watchdog):
		t.Fatal("timeout failure was not reported")
	}
	if failure.Panic || !errors.Is(failure.Err, context.DeadlineExceeded) {
		t.Fatalf("reported failure = %+v", failure)
	}
	awaitHistoryTest(t, sched, "budget-observed", 1)
	snapshot := findJobSnapshot(t, sched, "budget-observed")
	if len(snapshot.History) != 1 || snapshot.History[0].Outcome != OutcomeFailed || !strings.Contains(snapshot.History[0].Error, "timed out") {
		t.Fatalf("failure history = %+v", snapshot.History)
	}
}

func TestRunObserverReportsPanicAndPreservesFailure(t *testing.T) {
	h := newHarness(t)
	observer := &recordingObserver{begin: make(chan context.Context, 1), end: make(chan RunOutcome, 1)}
	h.sched.runObserver = observer
	reporterCalls := make(chan safego.Failure, 1)
	h.sched.reporter = safego.ReporterFunc(func(failure safego.Failure) string {
		reporterCalls <- failure
		return ""
	})
	entered := make(chan struct{})
	if err := h.sched.Register(Definition{
		ID: "panic-observed", Title: "Panic observed", Triggers: eventOn("panic", 0),
		Run: func(context.Context, Run) error {
			close(entered)
			panic("observer panic")
		},
	}); err != nil {
		t.Fatal(err)
	}
	h.sched.Emit("panic", nil)
	awaitSignalTest(t, entered, "run did not enter")
	_ = receiveContextTest(t, observer.begin)
	if got := receiveOutcomeTest(t, observer.end); got != RunFailed {
		t.Fatalf("observer outcome = %q, want %q", got, RunFailed)
	}
	assertNoOutcomeTest(t, observer.end)
	var failure safego.Failure
	select {
	case failure = <-reporterCalls:
	case <-time.After(watchdog):
		t.Fatal("panic failure was not reported")
	}
	var panicErr *safego.PanicError
	if !failure.Panic || !errors.As(failure.Err, &panicErr) || panicErr.Value != "observer panic" || failure.Stack == "" || failure.Stack != panicErr.Stack || !strings.Contains(failure.Stack, "TestRunObserverReportsPanicAndPreservesFailure") {
		t.Fatalf("reported panic = %+v, panic error = %+v", failure, panicErr)
	}
	awaitHistoryTest(t, h.sched, "panic-observed", 1)
	snapshot := findJobSnapshot(t, h.sched, "panic-observed")
	if len(snapshot.History) != 1 || snapshot.History[0].Outcome != OutcomeFailed || !strings.Contains(snapshot.History[0].Error, "observer panic") {
		t.Fatalf("failure history = %+v", snapshot.History)
	}
}

func findJobSnapshot(t *testing.T, sched *Scheduler, id string) Snapshot {
	t.Helper()
	for _, snapshot := range sched.Snapshot() {
		if snapshot.ID == id {
			return snapshot
		}
	}
	t.Fatalf("job %q was not registered", id)
	return Snapshot{}
}
