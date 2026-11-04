package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestStateSurvivesARestartAndTheStartupTriggerStaysOff(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "keep", Title: "Keep", Triggers: startupAt(0), Run: st.fn})
	h.nextRun(st)
	h.awaitJob("keep", succeeded)
	if err := h.sched.SetEnabled("keep", false); err != nil {
		t.Fatal(err)
	}

	h.restart()
	mustRegister(t, h, Definition{ID: "keep", Title: "Keep", Triggers: startupAt(0), Run: st.fn})
	snap := h.jobNamed("keep")
	if snap.Enabled || snap.Runs != 1 || len(snap.History) != 1 || snap.Status != StatusDisabled {
		t.Fatalf("after restart: %+v", snap)
	}
	noRunStarted(t, st)

	if err := h.sched.SetEnabled("keep", true); err != nil {
		t.Fatal(err)
	}
	if run := h.nextRun(st); run.Reason != ReasonStartup {
		t.Fatalf("reason after enabling = %q", run.Reason)
	}
}

func TestLastRunIsReadBackFromHistoryAfterARestart(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "f", Title: "F", StartDisabled: true, Run: st.fn})
	if err := h.sched.SetEnabled("f", true); err != nil {
		t.Fatal(err)
	}
	st.results <- errBoom
	if err := h.sched.RunNow("f"); err != nil {
		t.Fatal(err)
	}
	h.nextRun(st)
	h.awaitJob("f", failed)

	h.restart()
	mustRegister(t, h, Definition{ID: "f", Title: "F", StartDisabled: true, Run: st.fn})
	snap := h.jobNamed("f")
	if snap.Status != StatusFailed || !strings.Contains(snap.LastError, "boom") || snap.LastReason != ReasonManual || snap.Failures != 1 {
		t.Fatalf("restored last run: %+v", snap)
	}
}

func TestHistoryIsBoundedToTwentyEntries(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "many", Title: "Many", Run: st.fn})
	for range maxHistory + 5 {
		if err := h.sched.RunNow("many"); err != nil {
			t.Fatal(err)
		}
		h.nextRun(st)
	}
	h.awaitJob("many", func(s Snapshot) bool { return len(s.History) == maxHistory && s.Status == StatusSucceeded })
}

func TestMissedIntervalRunCaughtUpOnceAtStartup(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	hourly := []Trigger{{Kind: TriggerInterval, Every: time.Hour}}
	mustRegister(t, h, Definition{ID: "sync", Title: "Sync", Triggers: hourly, Run: st.fn})
	if err := h.sched.RunNow("sync"); err != nil {
		t.Fatal(err)
	}
	h.nextRun(st)
	h.awaitJob("sync", succeeded)

	h.closeAndSleep(2 * time.Hour)
	mustRegister(t, h, Definition{ID: "sync", Title: "Sync", Triggers: hourly, Run: st.fn})
	if run := h.nextRun(st); run.Reason != ReasonMissed {
		t.Fatalf("catch-up reason = %q", run.Reason)
	}
	h.awaitJob("sync", func(s Snapshot) bool { return len(s.History) == 2 })
}

func TestSkipPolicyDoesNotCatchUpMissedRuns(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	hourly := []Trigger{{Kind: TriggerInterval, Every: time.Hour, Missed: MissedSkip}}
	mustRegister(t, h, Definition{ID: "skip", Title: "Skip", Triggers: hourly, Run: st.fn})
	if err := h.sched.RunNow("skip"); err != nil {
		t.Fatal(err)
	}
	h.nextRun(st)
	h.awaitJob("skip", succeeded)

	h.closeAndSleep(2 * time.Hour)
	mustRegister(t, h, Definition{ID: "skip", Title: "Skip", Triggers: hourly, Run: st.fn})
	noRunStarted(t, st)
	h.advanceWhenDue(time.Hour)
	if run := h.nextRun(st); run.Reason != ReasonInterval {
		t.Fatalf("next reason = %q, want interval", run.Reason)
	}
}

// closeAndSleep stops the scheduler and store and moves the clock on, as a closed app would see the time pass.
func (h *harness) closeAndSleep(d time.Duration) {
	h.t.Helper()
	if err := h.sched.Close(); err != nil {
		h.t.Fatal(err)
	}
	if err := h.store.Close(); err != nil {
		h.t.Fatal(err)
	}
	h.clock.Advance(d)
	h.open()
}

func TestWakeEventRunsWakeJobsAfterASleep(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "onwake", Title: "On Wake", Triggers: eventOn("wake", 0), Run: st.fn})

	h.clock.Advance(time.Hour)
	noRunStarted(t, st)

	h.clock.Jump(25 * time.Hour)
	if run := h.nextRun(st); run.Reason != "event:wake" {
		t.Fatalf("wake reason = %q", run.Reason)
	}
	h.awaitJob("onwake", succeeded)
}

func TestHeartbeatGapsShorterThanTheThresholdAreNotSleep(t *testing.T) {
	if slept(time.Unix(0, 0), time.Unix(0, 0).Add(DefaultSleepGap), DefaultSleepGap) {
		t.Fatal("a gap equal to the threshold counted as sleep")
	}
	if !slept(time.Unix(0, 0), time.Unix(0, 0).Add(DefaultSleepGap+time.Second), DefaultSleepGap) {
		t.Fatal("a longer gap was not sleep")
	}
}

func TestSleepDoesNotBurstAnIntervalIntoManyRuns(t *testing.T) {
	h := newHarness(t)
	st := newStepper()
	mustRegister(t, h, Definition{ID: "every", Title: "Every", Triggers: []Trigger{{Kind: TriggerInterval, Every: 10 * time.Second}}, Run: st.fn})
	h.clock.AwaitTimerDueIn(10 * time.Second)
	h.clock.Jump(time.Hour)
	h.nextRun(st)
	h.awaitJob("every", func(s Snapshot) bool { return len(s.History) == 1 })
	noRunStarted(t, st)
}
