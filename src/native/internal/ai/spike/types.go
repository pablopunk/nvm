package spike

import (
	"context"
	"sync/atomic"
)

// Role names the author of a transcript message.
type Role string

// Transcript roles, matching the OpenAI chat format the backend accepts.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is one tool invocation the model asked for. Arguments is the JSON object text as the model produced it.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Message is one transcript entry. Assistant messages may carry ToolCalls; tool messages carry the ToolCallID they
// answer.
type Message struct {
	Role       Role
	Text       string
	ToolCalls  []ToolCall
	ToolCallID string
}

// UserMessage is what the user sent for one turn.
type UserMessage struct {
	Text string
}

// Session is one conversation transcript. Run appends to Messages. A session runs one turn at a time: a second Run
// while one is in flight ends at once with a session_busy error, as the 1.x chat does.
type Session struct {
	ID       string
	Messages []Message
	busy     atomic.Bool
}

func (s *Session) acquire() bool {
	return s.busy.CompareAndSwap(false, true)
}

func (s *Session) release() {
	s.busy.Store(false)
}

// EventKind names what one loop event reports.
type EventKind string

// Loop event kinds. Start is first; Done, Error and Aborted are terminal and appear exactly once, last.
const (
	EventStart     EventKind = "start"
	EventDelta     EventKind = "delta"
	EventToolStart EventKind = "tool_start"
	EventToolEnd   EventKind = "tool_end"
	EventError     EventKind = "error"
	EventAborted   EventKind = "aborted"
	EventDone      EventKind = "done"
)

// Event is one item of a turn's event stream. Only the fields for its Kind are set.
type Event struct {
	Kind    EventKind
	Text    string
	Tool    string
	CallID  string
	IsError bool
	Code    string
	Usage   Usage
}

// Agent runs one user turn against a session and streams its events.
type Agent interface {
	Run(ctx context.Context, session *Session, message UserMessage) <-chan Event
}
