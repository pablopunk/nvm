package agent

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Engine drives a turn's model loop outside this package, such as the pi host process. The host still owns every
// tool call the engine asks for: the policy, the user's approval and the run all stay here.
type Engine interface {
	// Run drives one reply over history, the messages the user's chat holds, ending with the user's message. It reports
	// the reply through run and returns nil when the reply is complete. It stops when ctx is cancelled.
	Run(ctx context.Context, run ExternalRun, history []backend.Message) error
}

// ExternalRun is what an engine reports a turn through.
type ExternalRun interface {
	// Step starts the engine's n-th model turn.
	Step(n int)
	// Delta and Reasoning stream a piece of the reply and of the model's reasoning.
	Delta(text string)
	Reasoning(text string)
	// NativeStart and NativeEnd report a tool the engine runs itself, such as a web search. The host neither judges nor
	// runs it; the transcript shows it.
	NativeStart(call CallInfo)
	NativeEnd(call CallInfo, failed bool)
	// RunCall asks the host to run a tool call: it checks the call, asks the user when the tool needs it, runs it, reports
	// it on the stream, and returns the text the model reads with how the call ended.
	RunCall(call backend.ToolCall) (text string, status CallStatus)
	// AddUsage adds the token count of the turn.
	AddUsage(usage Usage)
}

// StartExternal begins a turn driven by engine, with the events, cancellation and approvals of Start.
func StartExternal(parent context.Context, setup Setup, engine Engine, history []backend.Message) *Turn {
	ctx, cancel := context.WithCancelCause(parent)
	turn := &Turn{events: make(chan Event), cancel: cancel, waiting: map[string]chan bool{}}
	run := &turnRun{turn: turn, ctx: ctx, setup: setup.withDefaults(), messages: append([]backend.Message(nil), history...)}
	safego.Go("agent.turn.external", func(context.Context) { run.executeExternal(engine, history) })
	return turn
}

// executeExternal runs the engine and ends the turn with its outcome. The events channel closes when it returns.
func (r *turnRun) executeExternal(engine Engine, history []backend.Message) {
	defer close(r.turn.events)
	r.started = r.setup.Clock.Now()
	r.emit(Event{Kind: EventStart})
	err := engine.Run(r.ctx, externalRun{run: r}, history)
	switch {
	case r.ctx.Err() != nil:
		r.finish(r.endEvent())
	case err != nil:
		r.failWith(err)
	default:
		r.finish(Event{Kind: EventDone})
	}
}

// externalRun adapts a turn to the engine's view of it.
type externalRun struct {
	run *turnRun
}

func (e externalRun) Step(n int) {
	e.run.emit(Event{Kind: EventStep, Step: n})
}

func (e externalRun) Delta(text string) {
	e.run.emit(Event{Kind: EventDelta, Text: text})
}

func (e externalRun) Reasoning(text string) {
	e.run.emit(Event{Kind: EventReasoning, Text: text})
}

func (e externalRun) AddUsage(usage Usage) {
	e.run.usage.add(usage)
}

func (e externalRun) NativeStart(call CallInfo) {
	e.run.emit(Event{Kind: EventCalls, Calls: []backend.ToolCall{wireCall(call)}})
	e.run.emit(Event{Kind: EventToolStart, Call: call})
}

func (e externalRun) NativeEnd(call CallInfo, failed bool) {
	status, outcome := CallRan, "Done"
	if failed {
		status, outcome = CallFailed, "Failed"
	}
	e.run.emit(Event{Kind: EventToolEnd, Call: call, Status: status, Outcome: outcome})
}

func (e externalRun) RunCall(call backend.ToolCall) (string, CallStatus) {
	e.run.emit(Event{Kind: EventCalls, Calls: []backend.ToolCall{call}})
	outcome := e.run.runCall(call)
	return outcome.result, outcome.status
}

// wireCall is the transcript's record of a call the engine ran itself.
func wireCall(call CallInfo) backend.ToolCall {
	return backend.ToolCall{ID: call.ID, Type: "function", Function: backend.ToolFunction{Name: call.Name}}
}
