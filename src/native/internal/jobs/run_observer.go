package jobs

import "context"

// RunOutcome is the closed result set for an observed job execution.
type RunOutcome string

const (
	RunSucceeded RunOutcome = "success"
	RunFailed    RunOutcome = "failed"
	RunCancelled RunOutcome = "cancelled"
	RunTimedOut  RunOutcome = "timed_out"
)

// RunObserver is a host-owned port around actual work. It deliberately receives no job identity or payload.
type RunObserver interface {
	BeginRun(context.Context) (context.Context, func(RunOutcome))
}
