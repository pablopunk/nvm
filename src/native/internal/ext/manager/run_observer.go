package manager

import "context"

// RunOutcome is the domain-level result of one extension run.
type RunOutcome string

const (
	RunOutcomeSuccess   RunOutcome = "success"
	RunOutcomeFailed    RunOutcome = "failed"
	RunOutcomeCancelled RunOutcome = "cancelled"
	RunOutcomeTimedOut  RunOutcome = "timed_out"
)

// RunObserver optionally observes extension runs without receiving extension or result details.
type RunObserver interface {
	BeginRun(context.Context) (context.Context, func(RunOutcome))
}
