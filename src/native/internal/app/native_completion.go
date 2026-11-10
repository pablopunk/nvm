package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/diag/timing"
)

// nativeCommandRunner starts host work and reports its terminal outcome through complete.
type nativeCommandRunner func(context.Context, func(analytics.OperationOutcome))

// trackedNativeRun attaches one start-time journey/session and records only the terminal callback.
func trackedNativeRun(tracker *timing.Tracker, variant string, run nativeCommandRunner) func(context.Context) {
	return func(parent context.Context) {
		if tracker == nil {
			run(parent, func(analytics.OperationOutcome) {})
			return
		}
		ctx, completion := tracker.StartNativeCommand(parent, variant)
		run(ctx, completion.Finish)
	}
}
