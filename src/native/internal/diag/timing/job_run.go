package timing

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// StartJob begins a new background journey while retaining cancellation and parent values.
func (t *Tracker) StartJob(parent context.Context) (context.Context, *Completion) {
	ctx, _ := manager.ConsumeCommandRun(parent)
	ctx, completion := t.start(ctx, true)
	completion.job = true
	return ctx, completion
}

// JobRunObserver adapts scheduler boundaries to host-owned operation completions.
func JobRunObserver(tracker *Tracker) jobs.RunObserver {
	if tracker == nil {
		return nil
	}
	return jobRunObserver{tracker: tracker}
}

type jobRunObserver struct{ tracker *Tracker }

func (o jobRunObserver) BeginRun(ctx context.Context) (context.Context, func(jobs.RunOutcome)) {
	ctx, completion := o.tracker.StartJob(ctx)
	return ctx, func(outcome jobs.RunOutcome) {
		if mapped, ok := mapJobRunOutcome(outcome); ok {
			completion.Finish(mapped)
		}
	}
}

func mapJobRunOutcome(outcome jobs.RunOutcome) (analytics.OperationOutcome, bool) {
	switch outcome {
	case jobs.RunSkipped:
		return analytics.OutcomeBlocked, true
	case jobs.RunSucceeded:
		return analytics.OutcomeSuccess, true
	case jobs.RunFailed:
		return analytics.OutcomeFailed, true
	case jobs.RunCancelled:
		return analytics.OutcomeCancelled, true
	case jobs.RunTimedOut:
		return analytics.OutcomeTimedOut, true
	default:
		return "", false
	}
}
