package timing

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestJobRunObserverSerializesOnlyJobFailures(t *testing.T) {
	for _, outcome := range []jobs.RunOutcome{jobs.RunFailed, jobs.RunCancelled, jobs.RunTimedOut} {
		t.Run(string(outcome), func(t *testing.T) {
			set, client := newTestTracker(t)
			set.HTTP.Script(fake.Response{Status: 200})
			tracker := New(set.Clock, client)
			jobCtx, complete := JobRunObserver(tracker).BeginRun(context.Background())
			if jobCtx.Err() != nil {
				t.Fatal("job context unexpectedly cancelled")
			}
			if len(payloads(t, set)) != 0 {
				t.Fatal("job event sent before callback")
			}
			set.Clock.Advance(1*time.Millisecond + 400*time.Microsecond)
			complete(outcome)
			if err := client.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			events := payloads(t, set)
			if len(events) != 1 {
				t.Fatalf("events=%d, want job failure only", len(events))
			}
			props := events[0]
			for key, want := range map[string]any{
				"operation": "job.run", "outcome": string(outcome), "stage": "dispatch", "duration_ms": float64(1),
			} {
				if props[key] != want {
					t.Errorf("%s=%#v, want %#v", key, props[key], want)
				}
			}
			if _, ok := props["label"]; ok {
				t.Error("job event included a label")
			}
			for _, key := range []string{"boot_id", "journey_id", "action_id"} {
				id, ok := props[key].(string)
				if !ok || len(id) != 32 {
					t.Errorf("%s=%#v, want 32-character ID", key, props[key])
					continue
				}
				if _, err := hex.DecodeString(id); err != nil {
					t.Errorf("%s is not hexadecimal: %v", key, err)
				}
			}
		})
	}
}

func TestNilJobRunObserverIsAbsent(t *testing.T) {
	if JobRunObserver(nil) != nil {
		t.Fatal("nil tracker produced a job observer")
	}
}

func TestJobRunSuccessIsSuppressedAndCommandSuccessRemains(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	_, jobDone := JobRunObserver(tracker).BeginRun(context.Background())
	jobDone(jobs.RunSucceeded)
	_, command := tracker.Start(context.Background())
	command.Finish(analytics.OutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := payloads(t, set)
	if len(events) != 1 || events[0]["operation"] != "command.execute" || events[0]["outcome"] != "success" {
		t.Fatalf("events=%#v, want retained command success only", events)
	}
}

func TestStartJobCreatesFreshJourneyAndConsumesCommandMarker(t *testing.T) {
	set, client := newTestTracker(t)
	tracker := New(set.Clock, client)
	parentCtx, parent := tracker.Start(context.WithValue(context.Background(), contextCanaryKey{}, "private"))
	marked := manager.WithCommandRun(parentCtx)
	jobCtx, _ := tracker.StartJob(marked)
	parentIDs := parentCtx.Value(contextKey{}).(correlation)
	jobIDs := jobCtx.Value(contextKey{}).(correlation)
	if jobIDs.boot != parentIDs.boot || jobIDs.journey == parentIDs.journey || jobIDs.action == parentIDs.action {
		t.Fatal("job did not retain boot identity and create a fresh journey/action")
	}
	if got := jobCtx.Value(contextCanaryKey{}); got != "private" {
		t.Fatalf("parent value=%#v, want preserved value", got)
	}
	if _, consumed := manager.ConsumeCommandRun(jobCtx); consumed {
		t.Fatal("job context retained command marker")
	}
	parent.Finish(analytics.OutcomeSuccess)
}

func TestStartJobRetainsCancellationAndDeadline(t *testing.T) {
	set, _ := newTestTracker(t)
	tracker := New(set.Clock, nil)
	parent, cancel := context.WithCancel(context.Background())
	jobCtx, _ := tracker.StartJob(parent)
	cancel()
	if jobCtx.Err() != context.Canceled {
		t.Fatal("job context did not inherit cancellation")
	}
	deadlineParent, stop := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer stop()
	deadlineCtx, _ := tracker.StartJob(deadlineParent)
	if _, ok := deadlineCtx.Deadline(); !ok {
		t.Fatal("job context did not inherit deadline")
	}
}

func TestNestedMarkedCommandAndUnmarkedExtensionRun(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	jobCtx, _ := tracker.StartJob(context.Background())
	marked := manager.WithCommandRun(jobCtx)
	commandCtx, finish := ExtensionRunObserver(tracker).BeginRun(marked)
	finish(manager.RunOutcomeSuccess)
	if commandCtx.Value(contextKey{}).(correlation).journey != jobCtx.Value(contextKey{}).(correlation).journey {
		t.Fatal("nested command did not inherit job journey")
	}
	_, unmarkedFinish := ExtensionRunObserver(tracker).BeginRun(jobCtx)
	if unmarkedFinish != nil {
		t.Fatal("unmarked extension run unexpectedly observed")
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(payloads(t, set)); got != 1 {
		t.Fatalf("events=%d, want marked command completion only", got)
	}
}

func TestJobRunFinishOnceUnknownAndUnsupportedSink(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	_, finish := JobRunObserver(tracker).BeginRun(context.Background())
	var concurrent sync.WaitGroup
	for range 8 {
		concurrent.Add(1)
		go func() {
			defer concurrent.Done()
			finish(jobs.RunFailed)
		}()
	}
	concurrent.Wait()
	_, unknownFinish := JobRunObserver(tracker).BeginRun(context.Background())
	unknownFinish(jobs.RunOutcome("unknown"))
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(payloads(t, set)); got != 1 {
		t.Fatalf("events=%d, want one concurrent completion", got)
	}

	unsupported := New(set.Clock, commandOnlySink{})
	_, unsupportedFinish := JobRunObserver(unsupported).BeginRun(context.Background())
	unsupportedFinish(jobs.RunFailed)
}

type commandOnlySink struct{}

func (commandOnlySink) CommandCompleted(analytics.CommandCompletion) bool { return true }
