package timing

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

type viewOnlySink struct {
	commands, views int
	outcome         analytics.OperationOutcome
}

func (s *viewOnlySink) CommandCompleted(analytics.CommandCompletion) bool { s.commands++; return true }
func (s *viewOnlySink) ViewActionCompleted(c analytics.Completion) bool {
	s.views++
	s.outcome = c.Outcome
	return true
}

func TestViewActionObserverMarkersAndOptionalUsage(t *testing.T) {
	set := fake.NewSet()
	sink := &viewOnlySink{}
	tracker := New(set.Clock, sink)
	observer := ExtensionRunObserver(tracker)
	if _, finish := observer.BeginRun(context.Background()); finish != nil {
		t.Fatal("unmarked refresh recorded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	ctx = manager.WithViewActionRunOrigin(ctx, manager.ViewActionOrigin{})
	nested, finish := observer.BeginRun(ctx)
	if _, nestedFinish := observer.BeginRun(nested); nestedFinish != nil {
		t.Fatal("marker leaked into nested run")
	}
	cancel()
	if nested.Err() != context.Canceled {
		t.Fatal("cancellation lost")
	}
	finish(manager.RunOutcomeCancelled)
	finish(manager.RunOutcomeSuccess)
	if sink.views != 1 || sink.commands != 0 || sink.outcome != analytics.OutcomeCancelled {
		t.Fatalf("sink=%+v", sink)
	}
	both := manager.WithCommandRun(manager.WithViewActionRunOrigin(context.Background(), manager.ViewActionOrigin{}))
	nested, finish = observer.BeginRun(both)
	if _, nestedFinish := observer.BeginRun(nested); nestedFinish != nil {
		t.Fatal("markers leaked")
	}
	finish(manager.RunOutcomeSuccess)
	if sink.commands != 1 || sink.views != 1 {
		t.Fatalf("sink=%+v", sink)
	}
}

type viewUsageSink struct {
	viewOnlySink
	usage int
}

func (s *viewUsageSink) ViewActionUsed(analytics.Completion, string) bool { s.usage++; return true }
func TestViewActionInJobSuppressesUsage(t *testing.T) {
	set := fake.NewSet()
	sink := &viewUsageSink{}
	tracker := New(set.Clock, sink)
	ctx, _ := tracker.StartJob(context.Background())
	_, done := tracker.StartViewAction(ctx, manager.ViewActionOrigin{})
	done.Finish(analytics.OutcomeSuccess)
	if sink.views != 1 || sink.usage != 0 || sink.commands != 0 {
		t.Fatalf("sink=%+v", sink)
	}
}
