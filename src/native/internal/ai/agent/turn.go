package agent

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Setup is what a turn runs on. Model and Clock are required; zero values of the others select the defaults. A nil
// Tools offers no tools, and a nil Policy allows every call.
type Setup struct {
	Model          Model
	Tools          *Registry
	Policy         Policy
	Clock          platform.Clock
	MaxSteps       int
	ToolTimeout    time.Duration
	MaxOutputBytes int
	// TurnTimeout bounds the active time of a turn: model replies and tool runs, not the waits for the user.
	TurnTimeout time.Duration
}

func (s Setup) withDefaults() Setup {
	if s.Policy == nil {
		s.Policy = AllowAll{}
	}
	if s.MaxSteps <= 0 {
		s.MaxSteps = DefaultMaxSteps
	}
	if s.ToolTimeout <= 0 {
		s.ToolTimeout = DefaultToolTimeout
	}
	if s.MaxOutputBytes <= 0 {
		s.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if s.TurnTimeout <= 0 {
		s.TurnTimeout = DefaultTurnTimeout
	}
	return s
}

// Turn is one run of the loop. Its events come from Events in order and end with one terminal event, after which the
// channel closes. The consumer must drain Events; the loop waits on it.
type Turn struct {
	events chan Event
	cancel context.CancelCauseFunc

	mu      sync.Mutex
	waiting map[string]chan bool
}

// Start begins a turn over history, the messages the model sees, ending with the user's message. It returns at once;
// the loop runs on its own goroutine.
func Start(parent context.Context, setup Setup, history []backend.Message) *Turn {
	ctx, cancel := context.WithCancelCause(parent)
	turn := &Turn{events: make(chan Event), cancel: cancel, waiting: map[string]chan bool{}}
	run := &turnRun{turn: turn, ctx: ctx, setup: setup.withDefaults(), messages: append([]backend.Message(nil), history...)}
	safego.Go("agent.turn", func(context.Context) { run.execute() })
	return turn
}

// Events returns the stream of the turn.
func (t *Turn) Events() <-chan Event {
	return t.events
}

// Cancel stops the turn. A running tool sees its context end; calls that have not run are reported as cancelled.
func (t *Turn) Cancel() {
	t.cancel(context.Canceled)
}

// Decide answers the call of an EventApproval. It reports false when no approval waits for callID.
func (t *Turn) Decide(callID string, allow bool) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	waiting, ok := t.waiting[callID]
	if !ok {
		return false
	}
	delete(t.waiting, callID)
	waiting <- allow
	return true
}

// turnRun is the state of one turn while its loop goroutine runs. Only that goroutine touches it.
type turnRun struct {
	turn     *Turn
	ctx      context.Context
	setup    Setup
	messages []backend.Message
	usage    Usage
	finished bool
	started  time.Time
	// waited is the time the turn spent waiting for the user's answers, in nanoseconds. A running tool adds to it from
	// its own goroutine when it asks for an approval.
	waited atomic.Int64
}

func (r *turnRun) emit(event Event) {
	r.turn.events <- event
}

// activeTime is how long the turn has run, not counting the time it waited for the user's answers.
func (r *turnRun) activeTime() time.Duration {
	return r.setup.Clock.Now().Sub(r.started) - time.Duration(r.waited.Load())
}

// pastDeadline ends the turn with ErrTurnDeadline once its active time passes the limit. It reports whether it did.
func (r *turnRun) pastDeadline() bool {
	if r.activeTime() <= r.setup.TurnTimeout {
		return false
	}
	r.turn.cancel(ErrTurnDeadline)
	return true
}

// endEvent is the terminal event of a turn whose context ended: a deadline is an error, anything else a stop.
func (r *turnRun) endEvent() Event {
	if errors.Is(context.Cause(r.ctx), ErrTurnDeadline) {
		return Event{Kind: EventError, Err: ErrTurnDeadline}
	}
	return Event{Kind: EventAborted}
}

// finish sends the terminal event, once, with the usage of the turn.
func (r *turnRun) finish(event Event) {
	if r.finished {
		return
	}
	r.finished = true
	event.Usage = r.usage
	r.emit(event)
}

// execute runs the loop: a model turn, then the calls it asked for, until the model answers without calls or the step
// limit is reached. The events channel closes when it returns, even on a panic.
func (r *turnRun) execute() {
	defer close(r.turn.events)
	r.started = r.setup.Clock.Now()
	r.emit(Event{Kind: EventStart})
	for step := 1; step <= r.setup.MaxSteps; step++ {
		if r.pastDeadline() || r.ctx.Err() != nil {
			r.finish(r.endEvent())
			return
		}
		r.emit(Event{Kind: EventStep, Step: step})
		assistant, err := r.modelStep()
		if err != nil {
			r.failWith(err)
			return
		}
		r.messages = append(r.messages, assistant)
		if len(assistant.ToolCalls) == 0 {
			r.finish(Event{Kind: EventDone})
			return
		}
		r.emit(Event{Kind: EventCalls, Calls: assistant.ToolCalls})
		if !r.runCalls(assistant.ToolCalls) {
			return
		}
	}
	r.finish(Event{Kind: EventError, Err: ErrStepLimit})
}

// modelStep opens one model turn, streams it and returns the assistant message it made.
func (r *turnRun) modelStep() (backend.Message, error) {
	tools, err := r.setup.Tools.Wire()
	if err != nil {
		return backend.Message{}, err
	}
	stream, err := r.setup.Model.Stream(r.ctx, r.messages, tools)
	if err != nil {
		return backend.Message{}, err
	}
	defer func() { _ = stream.Close() }()
	return r.consume(stream)
}

// consume reads a model turn to its end, sending each text piece as a delta and assembling the tool calls.
func (r *turnRun) consume(stream ModelStream) (backend.Message, error) {
	var text strings.Builder
	calls := newCallAssembler()
	for {
		chunk, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return backend.Message{}, err
		}
		if chunk.Reasoning != "" {
			r.emit(Event{Kind: EventReasoning, Text: chunk.Reasoning})
		}
		if chunk.Text != "" {
			text.WriteString(chunk.Text)
			r.emit(Event{Kind: EventDelta, Text: chunk.Text})
		}
		if chunk.ToolDelta != nil {
			r.announceDraft(calls, *chunk.ToolDelta)
			calls.add(*chunk.ToolDelta)
		}
		if chunk.Usage != nil {
			r.usage.add(*chunk.Usage)
		}
		if r.pastDeadline() {
			return backend.Message{}, ErrTurnDeadline
		}
		if r.ctx.Err() != nil {
			return backend.Message{}, r.ctx.Err()
		}
	}
	return assistantMessage(text.String(), calls.list()), nil
}

// announceDraft emits EventCallDrafting the first time a tool call of the model step names its tool, so the transcript
// can say what the model is writing while its arguments stream.
func (r *turnRun) announceDraft(calls *callAssembler, delta backend.ToolCallDelta) {
	if delta.Name == "" || calls.named(delta.Index) {
		return
	}
	info := CallInfo{ID: delta.ID, Name: delta.Name, Label: delta.Name}
	if r.setup.Tools != nil {
		if tool, known := r.setup.Tools.Lookup(delta.Name); known {
			info.Label = tool.Label
		}
	}
	r.emit(Event{Kind: EventCallDrafting, Call: info})
}

// assistantMessage builds the assistant entry of a model turn. Content is left out when a turn only calls tools.
func assistantMessage(text string, calls []backend.ToolCall) backend.Message {
	message := backend.Message{Role: backend.RoleAssistant, ToolCalls: calls}
	if text != "" || len(calls) == 0 {
		message.Content = backend.TextContent(text)
	}
	return message
}

// failWith ends the turn for a failure of the model or a stream, or aborts it when the turn was cancelled.
func (r *turnRun) failWith(err error) {
	if r.ctx.Err() != nil {
		r.finish(r.endEvent())
		return
	}
	r.finish(Event{Kind: EventError, Err: err})
}
