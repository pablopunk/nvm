package jobs

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func mustRegister(t *testing.T, h *harness, def Definition) {
	t.Helper()
	if err := h.sched.Register(def); err != nil {
		t.Fatalf("register %s: %v", def.ID, err)
	}
}

func startupAt(delay time.Duration) []Trigger {
	return []Trigger{{Kind: TriggerStartup, Delay: delay}}
}

func eventOn(name string, debounce time.Duration) []Trigger {
	return []Trigger{{Kind: TriggerEvent, Event: name, Debounce: debounce}}
}

func succeeded(s Snapshot) bool { return s.Status == StatusSucceeded }
func failed(s Snapshot) bool    { return s.Status == StatusFailed }

// noRunStarted fails when a run was started that the test did not expect. It never waits.
func noRunStarted(t *testing.T, st *stepper) {
	t.Helper()
	select {
	case run := <-st.runs:
		t.Fatalf("unexpected run %s (%s)", run.ID, run.Reason)
	default:
	}
}

func TestStartupRunsOnceAfterItsDelay(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "a", Title: "A", Triggers: startupAt(5 * time.Second), Run: st.fn})
	noRunStarted(t, st)

	h.advanceWhenDue(5 * time.Second)
	if run := h.nextRun(st); run.Reason != ReasonStartup || run.ID != "a" {
		t.Fatalf("run = %+v, want startup of a", run)
	}
	h.awaitJob("a", succeeded)
}

func TestIntervalRunsEachPeriodAndNeverOverlaps(t *testing.T) {
	h := newHarness(t)
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	var mu sync.Mutex
	inFlight, peak := 0, 0
	st := newStepper()
	mustRegister(t, h, Definition{
		ID: "tick", Title: "Tick", Triggers: []Trigger{{Kind: TriggerInterval, Every: 10 * time.Second}},
		Run: func(ctx context.Context, run Run) error {
			mu.Lock()
			inFlight++
			peak = max(peak, inFlight)
			mu.Unlock()
			st.runs <- run
			<-gate
			mu.Lock()
			inFlight--
			mu.Unlock()
			return nil
		},
	})

	h.advanceWhenDue(10 * time.Second)
	h.nextRun(st)
	h.clock.Advance(10 * time.Second)
	gate <- struct{}{}
	h.awaitJob("tick", func(s Snapshot) bool { return len(s.History) >= 1 })

	mu.Lock()
	defer mu.Unlock()
	if peak != 1 {
		t.Fatalf("peak concurrency = %d, want 1", peak)
	}
}

func TestEventBurstRunsOnceAfterQuietAndNoLaterThanMaxWait(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{
		ID: "files", Title: "Files",
		Triggers: []Trigger{{Kind: TriggerEvent, Event: "files.changed", Debounce: 2 * time.Second, MaxWait: 5 * time.Second}},
		Run:      st.fn,
	})
	for i, path := range []string{"/a", "/b", "/c", "/d", "/e"} {
		h.sched.Emit("files.changed", Payload{"changedPaths": []string{path}})
		if i < 4 {
			h.clock.Advance(time.Second)
			noRunStarted(t, st)
		}
	}
	h.clock.Advance(time.Second)
	run := h.nextRun(st)
	if run.Reason != "event:files.changed" {
		t.Fatalf("reason = %q", run.Reason)
	}
	if got := run.Payload["changedPaths"].([]string); len(got) != 5 || got[0] != "/a" || got[4] != "/e" {
		t.Fatalf("changedPaths = %v", got)
	}
	noRunStarted(t, st)
}

func TestEventDuringRunIsQueuedAndRunsOnceAfterIt(t *testing.T) {
	h := newHarness(t)
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	st := newStepper()
	mustRegister(t, h, Definition{
		ID: "q", Title: "Q", Triggers: eventOn("poke", 0),
		Run: func(ctx context.Context, run Run) error {
			st.runs <- run
			<-gate
			return nil
		},
	})
	h.sched.Emit("poke", Payload{"changedPaths": []string{"/one"}})
	first := h.nextRun(st)

	h.sched.Emit("poke", Payload{"changedPaths": []string{"/two"}})
	h.sched.Emit("poke", Payload{"changedPaths": []string{"/three"}})
	h.awaitQueued("q", func(p Payload) bool {
		paths, _ := p["changedPaths"].([]string)
		return len(paths) == 2
	})
	gate <- struct{}{}
	second := h.nextRun(st)
	gate <- struct{}{}
	h.awaitJob("q", func(s Snapshot) bool { return len(s.History) == 2 })

	if got := first.Payload["changedPaths"].([]string); len(got) != 1 || got[0] != "/one" {
		t.Fatalf("first payload = %v", got)
	}
	if got := second.Payload["changedPaths"].([]string); len(got) != 2 || !slices.Contains(got, "/two") || !slices.Contains(got, "/three") {
		t.Fatalf("queued payload = %v", got)
	}
	noRunStarted(t, st)
}

func TestRunPastItsTimeoutGetsItsContextCancelled(t *testing.T) {
	h := newHarness(t)
	mustRegister(t, h, Definition{
		ID: "slow", Title: "Slow", Triggers: startupAt(0), Budget: Budget{Timeout: 3 * time.Second},
		Run: func(ctx context.Context, run Run) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	h.clock.AwaitTimerDueIn(3 * time.Second)
	h.clock.Advance(3 * time.Second)
	snap := h.awaitJob("slow", failed)
	if !strings.Contains(snap.LastError, "timed out") {
		t.Fatalf("last error = %q", snap.LastError)
	}
}

func TestPanickingRunIsAFailureAndTheSchedulerKeepsGoing(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	first := true
	mustRegister(t, h, Definition{
		ID: "p", Title: "P", Triggers: eventOn("go", 0),
		Run: func(ctx context.Context, run Run) error {
			st.runs <- run
			if first {
				first = false
				panic("kaboom")
			}
			return nil
		},
	})
	h.sched.Emit("go", nil)
	h.nextRun(st)
	snap := h.awaitJob("p", failed)
	if !strings.Contains(snap.LastError, "panicked") || snap.Consecutive != 1 {
		t.Fatalf("after panic: error %q, consecutive %d", snap.LastError, snap.Consecutive)
	}

	if err := h.sched.RunNow("p"); err != nil {
		t.Fatalf("manual run after panic: %v", err)
	}
	h.nextRun(st)
	h.awaitJob("p", succeeded)
}

func TestBackoffDefersAutomaticRunsButNotManualOnes(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "b", Title: "B", Triggers: eventOn("poke", 0), Run: st.fn})
	st.results <- errBoom
	h.sched.Emit("poke", nil)
	h.nextRun(st)
	h.awaitJob("b", failed)

	h.sched.Emit("poke", nil)
	h.awaitJob("b", func(s Snapshot) bool { return s.Status == StatusBackingOff })
	h.clock.Advance(DefaultBackoffInitial - time.Second)
	noRunStarted(t, st)

	h.clock.Advance(time.Second)
	if run := h.nextRun(st); run.Reason != ReasonRetry {
		t.Fatalf("retry reason = %q", run.Reason)
	}
	h.awaitJob("b", succeeded)
}

func TestManualRunIgnoresBackoff(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "m", Title: "M", Triggers: eventOn("poke", 0), Run: st.fn})
	st.results <- errBoom
	h.sched.Emit("poke", nil)
	h.nextRun(st)
	h.awaitJob("m", failed)

	if err := h.sched.RunNow("m"); err != nil {
		t.Fatal(err)
	}
	if run := h.nextRun(st); run.Reason != ReasonManual {
		t.Fatalf("reason = %q", run.Reason)
	}
}

func TestRepeatedFailuresHaltTheJobUntilItIsEnabledAgain(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "h", Title: "H", Halt: HaltPolicy{Failures: 3, Within: 10 * time.Minute}, Run: st.fn})
	for attempt := 1; attempt <= 3; attempt++ {
		st.results <- errBoom
		if err := h.sched.RunNow("h"); err != nil {
			t.Fatal(err)
		}
		h.nextRun(st)
		h.awaitJob("h", func(s Snapshot) bool { return len(s.History) == attempt })
	}
	snap := h.awaitJob("h", func(s Snapshot) bool { return s.Halted != "" })
	if snap.Status != StatusDisabled || snap.Failures != 3 || !strings.Contains(snap.Halted, "Stopped after 3 failures") {
		t.Fatalf("halted snapshot = %+v", snap)
	}
	if err := h.sched.RunNow("h"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("manual run of halted job: %v, want ErrDisabled", err)
	}

	if err := h.sched.SetEnabled("h", true); err != nil {
		t.Fatal(err)
	}
	if got := h.jobNamed("h"); got.Halted != "" || got.Status == StatusDisabled {
		t.Fatalf("enabling did not clear the halt: %+v", got)
	}
	if err := h.sched.RunNow("h"); err != nil {
		t.Fatalf("run after re-enable: %v", err)
	}
	h.nextRun(st)
}

func TestBudgetSkipsRunsOverTheWindowAndNeverCountsThemAsFailures(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{
		ID: "budget", Title: "Budget", Triggers: eventOn("tick", 0),
		Budget: Budget{MaxRuns: 2, Window: time.Minute}, Run: st.fn,
	})
	h.sched.Emit("tick", nil)
	h.nextRun(st)
	h.awaitJob("budget", func(s Snapshot) bool { return len(s.History) == 1 })
	h.sched.Emit("tick", nil)
	h.nextRun(st)
	h.awaitJob("budget", func(s Snapshot) bool { return len(s.History) == 2 })

	h.sched.Emit("tick", nil)
	snap := h.awaitJob("budget", func(s Snapshot) bool { return len(s.History) == 3 })
	if snap.History[0].Outcome != OutcomeSkipped || snap.Failures != 0 {
		t.Fatalf("over-budget run: history %+v, failures %d", snap.History[0], snap.Failures)
	}
	noRunStarted(t, st)

	h.clock.Advance(time.Minute)
	h.sched.Emit("tick", nil)
	h.nextRun(st)
	h.awaitJob("budget", succeeded)
}

func TestStartDisabledJobRunsOnlyAfterItIsEnabled(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "off", Title: "Off", StartDisabled: true, Triggers: startupAt(0), Run: st.fn})
	if h.jobNamed("off").Enabled {
		t.Fatal("start-disabled job reports enabled")
	}
	noRunStarted(t, st)

	if err := h.sched.SetEnabled("off", true); err != nil {
		t.Fatal(err)
	}
	if run := h.nextRun(st); run.Reason != ReasonStartup {
		t.Fatalf("reason = %q", run.Reason)
	}
	if err := h.sched.SetEnabled("off", false); err != nil {
		t.Fatal(err)
	}
	if got := h.jobNamed("off").Status; got != StatusDisabled {
		t.Fatalf("status after disabling = %s", got)
	}
}

func TestUnregisterDropsTheResultOfARunInProgress(t *testing.T) {
	h := newHarness(t)
	gate := make(chan struct{})
	st := newStepper()
	mustRegister(t, h, Definition{
		ID: "gone", Title: "Gone", Triggers: eventOn("go", 0),
		Run: func(ctx context.Context, run Run) error {
			st.runs <- run
			<-gate
			return nil
		},
	})
	h.sched.Emit("go", nil)
	h.nextRun(st)
	h.sched.Unregister("gone")
	close(gate)
	if h.sched.Has("gone") || len(h.sched.Snapshot()) != 0 {
		t.Fatal("unregistered job is still listed")
	}
}

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
