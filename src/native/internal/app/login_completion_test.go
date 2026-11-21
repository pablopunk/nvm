package app

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestLoginStartCompletionAfterUIStart(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cancel   bool
		startErr error
		want     analytics.OperationOutcome
		calls    int
	}{
		{name: "accepted", want: analytics.OutcomeSuccess, calls: 1},
		{name: "rejected", startErr: errors.New("in progress"), want: analytics.OutcomeFailed, calls: 1},
		{name: "deadline", startErr: context.DeadlineExceeded, want: analytics.OutcomeTimedOut, calls: 1},
		{name: "cancelled", cancel: true, want: analytics.OutcomeCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := make(completionUIQueue, 1)
			p := &App{uiQueue: queue}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			outcomes := make(chan analytics.OperationOutcome, 1)
			calls := 0
			p.startCheckedSignInWithCompletion(ctx, &recordingHost{}, func() error { calls++; return tc.startErr }, func(o analytics.OperationOutcome) { outcomes <- o })
			callback := <-queue
			select {
			case <-outcomes:
				t.Fatal("completed before UI start")
			default:
			}
			if tc.cancel {
				cancel()
			}
			callback()
			if got := <-outcomes; got != tc.want || calls != tc.calls {
				t.Fatalf("outcome %v calls %d", got, calls)
			}
			select {
			case <-outcomes:
				t.Fatal("completed twice")
			default:
			}
		})
	}
}

func TestTrackedLoginWithoutSessionBlocked(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200})
	client := analytics.New(analytics.Config{Endpoint: "https://collector.test", APIKey: analytics.IngestKey, Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/analytics", Consent: true})
	p := &App{commandTiming: timing.New(set.Clock, client)}
	host := &recordingHost{}
	cmd := p.signInOrOutCommands()[0]
	p.trackedRun(cmd)(host)
	flushNativeAnalytics(t, client)
	ops, uses := nativeCompletionEvents(t, set)
	if len(host.pushed) != 1 || len(ops) != 1 || ops[0]["outcome"] != "blocked" || len(uses) != 1 || uses[0]["outcome"] != "blocked" {
		t.Fatalf("views=%d operations=%v usage=%v", len(host.pushed), ops, uses)
	}
}

func TestLoginCompletionWaitsForStartReturn(t *testing.T) {
	queue := make(completionUIQueue, 1)
	p := &App{uiQueue: queue}
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	outcomes := make(chan analytics.OperationOutcome, 1)
	p.startCheckedSignInWithCompletion(context.Background(), &recordingHost{}, func() error { close(entered); <-release; return nil }, func(o analytics.OperationOutcome) { outcomes <- o })
	callback := <-queue
	go func() { defer close(finished); callback() }()
	<-entered
	select {
	case <-outcomes:
		t.Fatal("completed before start returned")
	default:
	}
	close(release)
	<-finished
	if got := <-outcomes; got != analytics.OutcomeSuccess {
		t.Fatalf("outcome = %v", got)
	}
}
