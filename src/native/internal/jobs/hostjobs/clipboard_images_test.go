package hostjobs

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// sweepSpy records each sweep the cleanup starts.
type sweepSpy struct {
	sweeps chan clipboard.GCRefs
}

func (s *sweepSpy) GC(refs clipboard.GCRefs) clipboard.GCResult {
	s.sweeps <- refs
	return clipboard.GCResult{Status: clipboard.GCSwept, Reason: "no-candidates"}
}

// refsStub is a trusted history with no pictures, enough for the cleanup to be wired.
type refsStub struct{}

func (refsStub) Trusted() bool                             { return true }
func (refsStub) ImageNames() (map[string]bool, error)      { return map[string]bool{}, nil }
func (refsStub) HasImage(name string) bool                 { return false }

func newCleanupScheduler(t *testing.T, clock *fake.Clock, spy *sweepSpy) *jobs.Scheduler {
	t.Helper()
	sched, err := jobs.New(jobs.Config{
		Clock:    clock,
		Reporter: safego.ReporterFunc(func(safego.Failure) string { return "" }),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sched.Close() })
	if err := sched.Register(ClipboardImageCleanup(spy, refsStub{})); err != nil {
		t.Fatal(err)
	}
	return sched
}

func TestCleanupSweepsThirtySecondsAfterStartThenHourly(t *testing.T) {
	clock := fake.NewClock()
	spy := &sweepSpy{sweeps: make(chan clipboard.GCRefs, 4)}
	sched := newCleanupScheduler(t, clock, spy)

	clock.AwaitTimerDueIn(ClipboardImageCleanupDelay)
	clock.Advance(ClipboardImageCleanupDelay)
	<-spy.sweeps
	awaitFinished(t, sched, ClipboardImageCleanupID, 1)

	clock.AwaitTimerDueIn(ClipboardImageCleanupEvery)
	clock.Advance(ClipboardImageCleanupEvery)
	<-spy.sweeps

	snap := findJob(t, sched, ClipboardImageCleanupID)
	if snap.Title != "Clipboard Image Cleanup" || snap.Owner != jobs.OwnerHost || snap.Scope != "cache" {
		t.Fatalf("job identity = %+v", snap)
	}
	if len(snap.Triggers) != 1 || snap.Triggers[0].Every != time.Hour {
		t.Fatalf("triggers = %+v", snap.Triggers)
	}
}

// awaitFinished waits until the job has finished n runs. An interval tick that lands during a run is dropped, so the
// test must not advance the clock again before the run is over.
func awaitFinished(t *testing.T, sched *jobs.Scheduler, id string, n int) {
	t.Helper()
	changes, cancel := sched.Subscribe()
	defer cancel()
	deadline := time.After(10 * time.Second)
	for {
		if len(findJob(t, sched, id).History) >= n {
			return
		}
		select {
		case <-changes:
		case <-deadline:
			t.Fatalf("job %s did not finish %d run(s)", id, n)
		}
	}
}

func findJob(t *testing.T, sched *jobs.Scheduler, id string) jobs.Snapshot {
	t.Helper()
	for _, snap := range sched.Snapshot() {
		if snap.ID == id {
			return snap
		}
	}
	t.Fatalf("no job %s", id)
	return jobs.Snapshot{}
}
