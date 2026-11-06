package timing

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// ExtensionRunObserver adapts manager run boundaries to safe operation completions.
func ExtensionRunObserver(tracker *Tracker) manager.RunObserver {
	if tracker == nil {
		return nil
	}
	return extensionRunObserver{tracker: tracker}
}

type extensionRunObserver struct{ tracker *Tracker }

func (o extensionRunObserver) BeginRun(ctx context.Context) (context.Context, func(manager.RunOutcome)) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, origin, marked := manager.ConsumeCommandOrigin(ctx)
	if !marked {
		return ctx, nil
	}
	ctx, completion := o.tracker.StartCommand(ctx, origin)
	return ctx, func(outcome manager.RunOutcome) {
		mapped, ok := mapRunOutcome(outcome)
		if ok {
			completion.Finish(mapped)
		}
	}
}

func mapRunOutcome(outcome manager.RunOutcome) (analytics.OperationOutcome, bool) {
	switch outcome {
	case manager.RunOutcomeSuccess:
		return analytics.OutcomeSuccess, true
	case manager.RunOutcomeFailed:
		return analytics.OutcomeFailed, true
	case manager.RunOutcomeCancelled:
		return analytics.OutcomeCancelled, true
	case manager.RunOutcomeTimedOut:
		return analytics.OutcomeTimedOut, true
	default:
		return "", false
	}
}
