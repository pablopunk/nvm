package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// watchdog bounds every wait on a goroutine. It only fails a test that would otherwise hang; it never paces one.
const watchdog = 10 * time.Second

// parkedHeartbeat keeps the wake heartbeat far away, so tests that advance the clock see no wake. Wake tests jump the
// clock past it.
const parkedHeartbeat = 24 * time.Hour

// harness is a scheduler on a fake clock and an in-memory file system, with the jobs document in a real store.
type harness struct {
	t     *testing.T
	clock *fake.Clock
	files *fake.FS
	store *store.Store
	sched *Scheduler
	// heartbeat overrides parkedHeartbeat for wake tests that need ordinary beats between sleeps.
	heartbeat time.Duration
}

// newHarness starts a fresh scheduler. Its failures go to a reporter that keeps nothing, so tests stay quiet.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, clock: fake.NewClock(), files: fake.NewFS()}
	h.open()
	t.Cleanup(func() { _ = h.sched.Close() })
	return h
}

// open starts a scheduler over the current store directory, opening the store first when needed.
func (h *harness) open() {
	h.t.Helper()
	st, err := store.Open(h.files, h.clock, "/cfg/nvm", store.WithDebounce(time.Hour))
	if err != nil {
		h.t.Fatal(err)
	}
	h.store = st
	sched, err := New(Config{
		Clock:     h.clock,
		Reporter:  safego.ReporterFunc(func(safego.Failure) string { return "" }),
		State:     StateDocument(st),
		Log:       logx.Default(),
		Heartbeat: positiveOr(h.heartbeat, parkedHeartbeat),
		SleepGap:  30 * time.Second,
	})
	if err != nil {
		h.t.Fatal(err)
	}
	h.sched = sched
}

// restart closes the scheduler (flushing its state) and starts a new one over the same files.
func (h *harness) restart() {
	h.t.Helper()
	if err := h.sched.Close(); err != nil {
		h.t.Fatal(err)
	}
	if err := h.store.Close(); err != nil {
		h.t.Fatal(err)
	}
	h.open()
}

// stepper records what a job's function saw: each run it started, and the error it should return next.
type stepper struct {
	runs    chan Run
	results chan error
}

func newStepper() *stepper {
	return &stepper{runs: make(chan Run, 32), results: make(chan error, 32)}
}

// fn is a job function that reports each run and returns the next queued result, or nil when none is queued.
func (st *stepper) fn(ctx context.Context, run Run) error {
	st.runs <- run
	select {
	case err := <-st.results:
		return err
	default:
		return nil
	}
}

// nextRun waits for the next run the function reports, failing the test when none starts.
func (h *harness) nextRun(st *stepper) Run {
	h.t.Helper()
	select {
	case run := <-st.runs:
		return run
	case <-time.After(watchdog):
		h.t.Fatal("no run started")
		return Run{}
	}
}

// advanceWhenDue waits until the clock has a timer due exactly d from now, then advances the clock by d.
func (h *harness) advanceWhenDue(d time.Duration) {
	h.t.Helper()
	h.clock.AwaitTimerDueIn(d)
	h.clock.Advance(d)
}

// jobNamed returns the snapshot of the job with id.
func (h *harness) jobNamed(id string) Snapshot {
	h.t.Helper()
	for _, snap := range h.sched.Snapshot() {
		if snap.ID == id {
			return snap
		}
	}
	h.t.Fatalf("no job %s", id)
	return Snapshot{}
}

// awaitJob waits until the job satisfies pred. The scheduler notifies subscribers on every change, so the wait is by
// change, not by time; the watchdog only bounds a test that never settles.
func (h *harness) awaitJob(id string, pred func(Snapshot) bool) Snapshot {
	h.t.Helper()
	changes, cancel := h.sched.Subscribe()
	defer cancel()
	deadline := time.After(watchdog)
	for {
		snap, found := findSnapshot(h.sched.Snapshot(), id)
		if found && pred(snap) {
			return snap
		}
		select {
		case <-changes:
		case <-deadline:
			h.t.Fatalf("job %s did not reach the expected state; last %+v", id, snap)
		}
	}
}

func findSnapshot(snaps []Snapshot, id string) (Snapshot, bool) {
	for _, snap := range snaps {
		if snap.ID == id {
			return snap, true
		}
	}
	return Snapshot{}, false
}

// errBoom is the failure the test functions return.
var errBoom = errors.New("boom")

// pendingEventsOf reports the payload queued behind the running run of a job, or nil when nothing is queued.
func (h *harness) queuedPayload(id string) Payload {
	h.sched.mu.Lock()
	defer h.sched.mu.Unlock()
	r, ok := h.sched.jobs[id]
	if !ok || r.pending == nil {
		return nil
	}
	return r.pending.payload
}

// awaitQueued waits until the run queued behind the running one carries pred's payload.
func (h *harness) awaitQueued(id string, pred func(Payload) bool) {
	h.t.Helper()
	changes, cancel := h.sched.Subscribe()
	defer cancel()
	deadline := time.After(watchdog)
	for {
		if pred(h.queuedPayload(id)) {
			return
		}
		select {
		case <-changes:
		case <-deadline:
			h.t.Fatalf("job %s never queued the expected run", id)
		}
	}
}
