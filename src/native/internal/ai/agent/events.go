package agent

import (
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// EventKind names what one event of a turn reports.
type EventKind string

// The event kinds. Start comes first; Done, Error and Aborted are terminal and appear exactly once, last.
const (
	EventStart     EventKind = "start"
	EventStep      EventKind = "step"
	EventDelta     EventKind = "delta"
	EventCalls     EventKind = "calls"
	EventToolStart EventKind = "tool_start"
	EventApproval  EventKind = "approval"
	EventToolEnd   EventKind = "tool_end"
	EventError     EventKind = "error"
	EventAborted   EventKind = "aborted"
	EventDone      EventKind = "done"
)

// CallStatus says how one tool call ended.
type CallStatus string

// The call statuses. A call that did not run says why in Outcome; every call ends in exactly one of them.
const (
	CallRan       CallStatus = "ran"
	CallFailed    CallStatus = "failed"
	CallRefused   CallStatus = "refused"
	CallCancelled CallStatus = "cancelled"
)

// CallInfo identifies a tool call on the event stream. Input is the tool's one-line summary of the arguments.
type CallInfo struct {
	ID    string
	Name  string
	Label string
	Input string
}

// Event is one item of a turn's stream. Only the fields of its Kind are set.
type Event struct {
	Kind EventKind
	// Step numbers the model turn of EventStep, from 1.
	Step int
	// Text is a piece of reply text, set on EventDelta.
	Text string
	// Calls are the tool calls a model turn ended with, on EventCalls.
	Calls []backend.ToolCall
	// Call describes the tool call of EventToolStart, EventApproval and EventToolEnd.
	Call CallInfo
	// Status and Result are the outcome of EventToolEnd. Result is the text the model reads, capped.
	Status CallStatus
	Result string
	// Outcome is the one-line result the transcript shows for the call.
	Outcome string
	// Err is the failure of EventError; its type says which failure it is.
	Err error
	// Usage is the token count of all model turns, set on EventDone.
	Usage Usage
}

// Sentinel errors of the loop. Model and backend failures keep their own types and reach the caller unchanged.
var (
	// ErrStepLimit ends a turn that used every model turn without a final answer.
	ErrStepLimit = errors.New("agent: the model used every step without a final answer")
	// ErrTurnInterrupted is the failure of a turn whose loop stopped without a terminal event.
	ErrTurnInterrupted = errors.New("agent: the turn stopped unexpectedly")
)

// Default bounds of a turn. Zero options select them.
const (
	DefaultMaxSteps       = 8
	DefaultToolTimeout    = 30 * time.Second
	DefaultMaxOutputBytes = 16 * 1024
)
