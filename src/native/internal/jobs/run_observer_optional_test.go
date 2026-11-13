package jobs

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerWithoutRunObserverCompletesAndRecordsHistory(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{}, 1)
	if err := h.sched.Register(Definition{ID: "legacy", Title: "Legacy", Run: func(context.Context, Run) error {
		started <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if err := h.sched.RunNow("legacy"); err != nil {
		t.Fatal(err)
	}
	awaitSignalTest(t, started, "legacy job did not run")
	awaitHistoryTest(t, h.sched, "legacy", 1)
	snapshot, found := findSnapshot(h.sched.Snapshot(), "legacy")
	if !found || snapshot.History[0].Outcome != OutcomeSucceeded {
		t.Fatalf("legacy snapshot = %+v, found %t", snapshot, found)
	}
}

func TestSchedulerObserverAllowsNilFinishAndSkipsOverBudgetRun(t *testing.T) {
	h := newHarness(t)
	observer := &nilFinishObserver{begun: make(chan context.Context, 2)}
	h.sched.runObserver = observer
	started := make(chan struct{}, 2)
	if err := h.sched.Register(Definition{
		ID: "budgeted", Title: "Budgeted", Budget: Budget{MaxRuns: 1, Window: time.Minute},
		Run: func(context.Context, Run) error { started <- struct{}{}; return nil },
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.sched.RunNow("budgeted"); err != nil {
		t.Fatal(err)
	}
	awaitSignalTest(t, started, "budgeted job did not run")
	awaitHistoryTest(t, h.sched, "budgeted", 1)
	if ctx := receiveContextTest(t, observer.begun); ctx.Value(observerContextKey{}) != "observed" {
		t.Fatal("observer-derived context was not passed to the job")
	}

	if err := h.sched.RunNow("budgeted"); err != nil {
		t.Fatal(err)
	}
	awaitHistoryTest(t, h.sched, "budgeted", 2)
	select {
	case <-started:
		t.Fatal("over-budget run invoked job body")
	default:
	}
	select {
	case <-observer.begun:
		t.Fatal("over-budget run invoked observer")
	default:
	}
	snapshot, found := findSnapshot(h.sched.Snapshot(), "budgeted")
	if !found || snapshot.History[0].Outcome != OutcomeSkipped || snapshot.History[1].Outcome != OutcomeSucceeded {
		t.Fatalf("budgeted snapshot = %+v, found %t", snapshot, found)
	}
}

type nilFinishObserver struct{ begun chan context.Context }

func (o *nilFinishObserver) BeginRun(ctx context.Context) (context.Context, func(RunOutcome)) {
	derived := context.WithValue(ctx, observerContextKey{}, "observed")
	o.begun <- derived
	return derived, nil
}
