package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunNowRefusesUnknownAndDisabledJobs(t *testing.T) {
	h := newHarness(t)
	if err := h.sched.RunNow("nope"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("unknown: %v", err)
	}
	mustRegister(t, h, Definition{ID: "d", Title: "D", StartDisabled: true, Run: newStepper().fn})
	if err := h.sched.RunNow("d"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled: %v", err)
	}
}

func TestSnapshotIsOrderedByTitleAndReportsTriggers(t *testing.T) {
	h := newHarness(t)
	noop := func(context.Context, Run) error { return nil }
	mustRegister(t, h, Definition{ID: "b", Title: "Beta", Triggers: startupAt(0), Run: noop})
	mustRegister(t, h, Definition{ID: "a", Title: "Alpha", Triggers: eventOn("x", 0), Run: noop})
	snaps := h.sched.Snapshot()
	if len(snaps) != 2 || snaps[0].Title != "Alpha" || snaps[1].Owner != OwnerHost {
		t.Fatalf("snapshots = %+v", snaps)
	}
	if snaps[0].Triggers[0].Kind != TriggerEvent || snaps[0].Triggers[0].Event != "x" {
		t.Fatalf("triggers = %+v", snaps[0].Triggers)
	}
}

func TestClearErrorEndsBackoffAndTheFailedStatus(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "c", Title: "C", Triggers: eventOn("poke", 0), Run: st.fn})
	st.results <- errBoom
	h.sched.Emit("poke", nil)
	h.nextRun(st)
	h.awaitJob("c", failed)

	if err := h.sched.ClearError("c"); err != nil {
		t.Fatal(err)
	}
	snap := h.jobNamed("c")
	if snap.Status != StatusIdle || snap.LastError != "" || !snap.BackoffUntil.IsZero() {
		t.Fatalf("after clear: %+v", snap)
	}
	h.sched.Emit("poke", nil)
	h.nextRun(st)
}

func TestCloseStopsTriggersAndRefusesNewWork(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "c", Title: "C", Triggers: []Trigger{{Kind: TriggerInterval, Every: 10 * time.Second}}, Run: st.fn})
	if err := h.sched.Close(); err != nil {
		t.Fatal(err)
	}
	h.clock.Advance(time.Minute)
	noRunStarted(t, st)
	if err := h.sched.Register(Definition{ID: "x", Title: "X", Run: st.fn}); !errors.Is(err, ErrClosed) {
		t.Fatalf("register after close: %v", err)
	}
	if err := h.sched.RunNow("c"); !errors.Is(err, ErrClosed) {
		t.Fatalf("run after close: %v", err)
	}
}

func TestSubscribersHearAboutChanges(t *testing.T) {
	h := newHarness(t)
	changes, cancel := h.sched.Subscribe()
	defer cancel()
	mustRegister(t, h, Definition{ID: "s", Title: "S", Run: newStepper().fn})
	select {
	case <-changes:
	case <-time.After(watchdog):
		t.Fatal("no notification after register")
	}
}
