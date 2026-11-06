package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/update"
)

type blockingUpdateEngine struct {
	entered chan struct{}
	release chan struct{}
	answer  update.State
	check   func(context.Context) (update.Offer, bool, error)
}

func (e *blockingUpdateEngine) Enabled() bool                                     { return true }
func (e *blockingUpdateEngine) SetPrereleases(bool)                               {}
func (e *blockingUpdateEngine) Relaunch()                                         {}
func (e *blockingUpdateEngine) Install(context.Context, func(int64, int64)) error { return nil }
func (e *blockingUpdateEngine) Check(ctx context.Context) (update.Offer, bool, error) {
	close(e.entered)
	if e.release != nil {
		select {
		case <-e.release:
		case <-ctx.Done():
			return update.Offer{}, false, ctx.Err()
		}
	}
	if e.check != nil {
		return e.check(ctx)
	}
	return update.Offer{}, false, nil
}

func newCompletionUpdater(t *testing.T, engine update.Engine) *update.Service {
	t.Helper()
	return update.NewService(update.Options{Engine: engine, Clock: fake.NewClock(), Current: "2.0.0"})
}

type completionUIQueue chan func()

func (q completionUIQueue) Update(fn func()) { q <- fn }

func TestUpdateCheckCompletionWaitsForCheck(t *testing.T) {
	p := goldenPalette(t)
	p.updatesCtx, p.stopUpdates = context.WithCancel(context.Background())
	t.Cleanup(p.stopUpdates)
	queue := make(completionUIQueue, 1)
	p.uiQueue = queue
	engine := &blockingUpdateEngine{entered: make(chan struct{}), release: make(chan struct{})}
	p.updates = newCompletionUpdater(t, engine)
	reports := make(chan string, 1)
	completions := make(chan analytics.OperationOutcome, 2)
	p.checkForUpdatesWithCompletion(context.Background(), func(message string) { reports <- message }, func(outcome analytics.OperationOutcome) { completions <- outcome })
	<-engine.entered
	select {
	case outcome := <-completions:
		t.Fatalf("completion before Check returned: %s", outcome)
	default:
	}
	close(engine.release)
	if got := <-completions; got != analytics.OutcomeSuccess {
		t.Fatalf("completion = %q, want %q", got, analytics.OutcomeSuccess)
	}
	queuedReport := <-queue
	queuedReport()
	if got := <-reports; got != "Nevermind is up to date" {
		t.Fatalf("report = %q", got)
	}
	select {
	case extra := <-completions:
		t.Fatalf("duplicate completion: %s", extra)
	default:
	}
}

func TestUpdateCheckCompletionIgnoresBusyCheck(t *testing.T) {
	p := goldenPalette(t)
	p.updatesCtx, p.stopUpdates = context.WithCancel(context.Background())
	t.Cleanup(p.stopUpdates)
	queue := make(completionUIQueue, 1)
	p.uiQueue = queue
	engine := &blockingUpdateEngine{entered: make(chan struct{}), release: make(chan struct{})}
	p.updates = newCompletionUpdater(t, engine)
	completions := make(chan analytics.OperationOutcome, 2)
	p.checkForUpdatesWithCompletion(context.Background(), func(string) {}, func(outcome analytics.OperationOutcome) { completions <- outcome })
	<-engine.entered
	busyCompletion := make(chan analytics.OperationOutcome, 1)
	p.checkForUpdatesWithCompletion(context.Background(), func(string) {}, func(outcome analytics.OperationOutcome) { busyCompletion <- outcome })
	select {
	case got := <-busyCompletion:
		t.Fatalf("busy check completed command: %s", got)
	default:
	}
	close(engine.release)
	if got := <-completions; got != analytics.OutcomeSuccess {
		t.Fatalf("owned check completion = %q", got)
	}
}

func TestUpdateCheckCompletionCancelsOnQuit(t *testing.T) {
	p := goldenPalette(t)
	queue := make(completionUIQueue, 1)
	p.uiQueue = queue
	engine := &blockingUpdateEngine{entered: make(chan struct{}), release: make(chan struct{})}
	p.updates = newCompletionUpdater(t, engine)
	p.updatesCtx, p.stopUpdates = context.WithCancel(context.Background())
	t.Cleanup(p.stopUpdates)
	completions := make(chan analytics.OperationOutcome, 1)
	p.checkForUpdatesWithCompletion(context.Background(), func(string) {}, func(outcome analytics.OperationOutcome) { completions <- outcome })
	<-engine.entered
	p.beginQuit(nil)
	if got := <-completions; got != analytics.OutcomeCancelled {
		t.Fatalf("quit completion = %q, want cancelled", got)
	}
}

func TestUpdateCheckCompletionOutcomes(t *testing.T) {
	tests := []struct {
		name   string
		ctxErr error
		state  update.State
		want   analytics.OperationOutcome
	}{
		{name: "current", state: update.State{Status: update.StatusCurrent}, want: analytics.OutcomeSuccess},
		{name: "available", state: update.State{Status: update.StatusAvailable}, want: analytics.OutcomeSuccess},
		{name: "unsupported", state: update.State{Status: update.StatusUnsupported}, want: analytics.OutcomeSuccess},
		{name: "check failed", state: update.State{Status: update.StatusFailed, Stage: update.StageCheck}, want: analytics.OutcomeFailed},
		{name: "cancelled", ctxErr: context.Canceled, state: update.State{Status: update.StatusCurrent}, want: analytics.OutcomeCancelled},
		{name: "deadline", ctxErr: context.DeadlineExceeded, state: update.State{Status: update.StatusCurrent}, want: analytics.OutcomeTimedOut},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateCheckOutcome(tt.ctxErr, tt.state); got != tt.want {
				t.Fatalf("outcome = %q, want %q", got, tt.want)
			}
		})
	}
}
