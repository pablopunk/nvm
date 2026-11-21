package hostjobs

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// TestAIConversationCleanupSweepsOneSecondAfterStartThenHourly checks the sweep runs at launch and again each hour, and
// that the job is a host job named for the conversations.
func TestAIConversationCleanupSweepsOneSecondAfterStartThenHourly(t *testing.T) {
	clock := fake.NewClock()
	sweeps := make(chan struct{}, 4)
	sched, err := jobs.New(jobs.Config{Clock: clock, Reporter: safego.ReporterFunc(func(safego.Failure) string { return "" })})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sched.Close() })
	if err := sched.Register(AIConversationCleanup(func(context.Context) error {
		sweeps <- struct{}{}
		return nil
	})); err != nil {
		t.Fatal(err)
	}

	clock.AwaitTimerDueIn(AIConversationCleanupDelay)
	clock.Advance(AIConversationCleanupDelay)
	<-sweeps
	awaitFinished(t, sched, AIConversationCleanupID, 1)

	clock.AwaitTimerDueIn(AIConversationCleanupEvery)
	clock.Advance(AIConversationCleanupEvery)
	<-sweeps

	snap := findJob(t, sched, AIConversationCleanupID)
	if snap.Title != "AI Conversation Cleanup" || snap.Owner != jobs.OwnerHost || snap.Scope != "ai" {
		t.Fatalf("job identity = %+v", snap)
	}
}
