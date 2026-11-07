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
	// Session is an opaque start-time attribution token. Jobs ignore it.
	Session SessionSnapshot
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
	return c.withSessionCapture(completion.Session, func(sessionID string) bool {
		return c.operationCompleted(completion, "command.execute", "Extension command", "invoke", false, sessionID)
	})
}

// ViewActionCompleted records a host-owned extension view action completion.
func (c *Client) ViewActionCompleted(completion Completion) bool {
	return c.withSessionCapture(completion.Session, func(sessionID string) bool {
		return c.operationCompleted(completion, "view.action", "Extension view action", "invoke", false, sessionID)
	})
}

// JobCompleted records a background job completion, omitting routine successes.
func (c *Client) JobCompleted(completion JobCompletion) bool {
	completion.Session = SessionSnapshot{}
	return c.operationCompleted(completion, "job.run", "", "dispatch", true, "")
}

// withSessionCapture holds the session transition lock through validation and queue acceptance.
// Lock order is sessionMu then queue mu, matching consent withdrawal and discard.
func (c *Client) withSessionCapture(snapshot SessionSnapshot, capture func(string) bool) bool {
	if c == nil {
		return false
	}
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if snapshot.id == "" || snapshot.epoch != c.session.epoch || !c.consent.Load() {
		return capture("")
	}
	return capture(snapshot.id)
}

func (c *Client) operationCompleted(completion Completion, operation, label, stage string, suppressSuccess bool, sessionID string) bool {
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
	if sessionID != "" {
		props["$session_id"] = sessionID
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
