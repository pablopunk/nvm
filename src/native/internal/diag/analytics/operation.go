package analytics

import (
	"regexp"
	"time"
)

// OperationOutcome is the established closed set for diagnostic operation results.
type OperationOutcome string

const (
	OutcomeSuccess     OperationOutcome = "success"
	OutcomeFailed      OperationOutcome = "failed"
	OutcomeCancelled   OperationOutcome = "cancelled"
	OutcomeBlocked     OperationOutcome = "blocked"
	OutcomeTimedOut    OperationOutcome = "timed_out"
	OutcomeInterrupted OperationOutcome = "interrupted"
	OutcomeUnknown     OperationOutcome = "unknown"
)

// Completion contains host-owned context for an operation completion.
type Completion struct {
	Outcome   OperationOutcome
	Duration  time.Duration
	BootID    string
	JourneyID string
	ActionID  string
}

// CommandCompletion preserves the command completion API.
type CommandCompletion = Completion

// JobCompletion contains host-owned context for a background job completion.
type JobCompletion = Completion

var diagnosticIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

func validOperationOutcome(outcome OperationOutcome) bool {
	switch outcome {
	case OutcomeSuccess, OutcomeFailed, OutcomeCancelled, OutcomeBlocked, OutcomeTimedOut, OutcomeInterrupted, OutcomeUnknown:
		return true
	default:
		return false
	}
}

// CommandCompleted records a safe, fixed-label completion for an extension command.
func (c *Client) CommandCompleted(completion CommandCompletion) bool {
	return c.operationCompleted(completion, "command.execute", "Extension command", "invoke", false)
}

// JobCompleted records a background job completion, omitting routine successes.
func (c *Client) JobCompleted(completion JobCompletion) bool {
	return c.operationCompleted(completion, "job.run", "", "dispatch", true)
}

func (c *Client) operationCompleted(completion Completion, operation, label, stage string, suppressSuccess bool) bool {
	if c == nil || !validOperationOutcome(completion.Outcome) ||
		!diagnosticIDPattern.MatchString(completion.BootID) ||
		!diagnosticIDPattern.MatchString(completion.JourneyID) ||
		!diagnosticIDPattern.MatchString(completion.ActionID) {
		return false
	}
	if suppressSuccess && completion.Outcome == OutcomeSuccess {
		return false
	}
	duration := completion.Duration
	if duration < 0 {
		duration = 0
	}
	props := map[string]any{
		"operation":   operation,
		"outcome":     string(completion.Outcome),
		"stage":       stage,
		"duration_ms": int64(duration/time.Millisecond) + boolToInt(duration%time.Millisecond >= time.Millisecond/2),
		"boot_id":     completion.BootID,
		"journey_id":  completion.JourneyID,
		"action_id":   completion.ActionID,
	}
	if label != "" {
		props["label"] = label
	}
	for key, value := range c.releaseProperties() {
		props[key] = value
	}
	return c.capture(EventOperationCompleted, props)
}

func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
