package spike

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// Event codes the loop itself raises. Model failures carry the model's own code.
const (
	CodeIterationLimit = "iteration_limit"
	CodeSessionBusy    = "session_busy"
	CodeModelFailed    = "model_failed"
	CodeMockExhausted  = "mock_exhausted"
)

const (
	defaultMaxModelTurns      = 8
	defaultToolTimeout        = 30 * time.Second
	defaultMaxToolOutputBytes = 50 * 1024
)

const abortedByCallerText = "Tool call was not run because the turn was cancelled."

// Options tune the loop. Zero values select the defaults.
type Options struct {
	MaxModelTurns      int
	ToolTimeout        time.Duration
	MaxToolOutputBytes int
	Policy             Policy
}

type loopAgent struct {
	model    Model
	registry *Registry
	options  Options
}

// NewAgent returns the prototype loop over model and registry. A nil Policy allows every call.
func NewAgent(model Model, registry *Registry, options Options) Agent {
	if options.Policy == nil {
		options.Policy = allowAll{}
	}
	if options.MaxModelTurns <= 0 {
		options.MaxModelTurns = defaultMaxModelTurns
	}
	if options.ToolTimeout <= 0 {
		options.ToolTimeout = defaultToolTimeout
	}
	if options.MaxToolOutputBytes <= 0 {
		options.MaxToolOutputBytes = defaultMaxToolOutputBytes
	}
	return &loopAgent{model: model, registry: registry, options: options}
}

// Run starts one turn in its own goroutine. The returned channel closes after the terminal event; the goroutine
// exits before the close, so a drained channel means no loop goroutine is left.
func (a *loopAgent) Run(ctx context.Context, session *Session, message UserMessage) <-chan Event {
	events := make(chan Event)
	go func() {
		defer close(events)
		turn := &turnRun{agent: a, ctx: ctx, session: session, out: events}
		turn.execute(message)
	}()
	return events
}

// turnRun holds the state of one user turn while its goroutine runs.
type turnRun struct {
	agent    *loopAgent
	ctx      context.Context
	session  *Session
	out      chan<- Event
	usage    Usage
	finished bool
}

func (r *turnRun) emit(event Event) {
	r.out <- event
}

func (r *turnRun) finish(event Event) {
	if r.finished {
		return
	}
	r.finished = true
	event.Usage = r.usage
	r.out <- event
}

func (r *turnRun) finishAborted() {
	r.finish(Event{Kind: EventAborted})
}

func (r *turnRun) execute(message UserMessage) {
	if !r.session.acquire() {
		r.finish(Event{Kind: EventError, Code: CodeSessionBusy, Text: "this session is already responding"})
		return
	}
	defer r.session.release()
	r.session.Messages = append(r.session.Messages, Message{Role: RoleUser, Text: message.Text})
	r.emit(Event{Kind: EventStart})
	for turn := 0; turn < r.agent.options.MaxModelTurns; turn++ {
		if r.ctx.Err() != nil {
			r.finishAborted()
			return
		}
		assistant, err := r.modelTurn()
		if err != nil {
			r.failWith(err)
			return
		}
		r.session.Messages = append(r.session.Messages, assistant)
		if len(assistant.ToolCalls) == 0 {
			r.finish(Event{Kind: EventDone})
			return
		}
		if !r.runToolCalls(assistant.ToolCalls) {
			return
		}
	}
	r.finish(Event{
		Kind: EventError,
		Code: CodeIterationLimit,
		Text: fmt.Sprintf("the agent stopped after %d model turns", r.agent.options.MaxModelTurns),
	})
}

func (r *turnRun) modelTurn() (Message, error) {
	request := ModelRequest{Messages: append([]Message(nil), r.session.Messages...), Tools: r.agent.registry.Specs()}
	stream, err := r.agent.model.Stream(r.ctx, request)
	if err != nil {
		return Message{}, err
	}
	defer closeStream(stream)
	return r.consume(stream)
}

func (r *turnRun) consume(stream ModelStream) (Message, error) {
	var text strings.Builder
	calls := newCallAssembler()
	for {
		chunk, err := stream.Next()
		if isStreamEnd(err) {
			break
		}
		if err != nil {
			return Message{}, err
		}
		if chunk.Text != "" {
			text.WriteString(chunk.Text)
			r.emit(Event{Kind: EventDelta, Text: chunk.Text})
		}
		if chunk.ToolDelta != nil {
			calls.add(*chunk.ToolDelta)
		}
		if chunk.Usage != nil {
			r.usage.add(*chunk.Usage)
		}
		if r.ctx.Err() != nil {
			return Message{}, r.ctx.Err()
		}
	}
	return Message{Role: RoleAssistant, Text: text.String(), ToolCalls: calls.list()}, nil
}

// failWith ends the turn for a model or stream error, mapping a cancelled context to aborted.
func (r *turnRun) failWith(err error) {
	if r.ctx.Err() != nil || errors.Is(err, context.Canceled) {
		r.finishAborted()
		return
	}
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		r.finish(Event{Kind: EventError, Code: modelErr.Code, Text: modelErr.Message})
		return
	}
	r.finish(Event{Kind: EventError, Code: CodeModelFailed, Text: err.Error()})
}

// runToolCalls executes calls in order and appends one tool message per call. It returns false when the turn was
// cancelled; every call that did not run is then answered so the transcript stays valid for the next turn.
func (r *turnRun) runToolCalls(calls []ToolCall) bool {
	for index, call := range calls {
		if r.ctx.Err() != nil {
			r.abandonCalls(calls[index:])
			r.finishAborted()
			return false
		}
		r.emit(Event{Kind: EventToolStart, Tool: call.Name, CallID: call.ID})
		outcome := r.agent.runTool(r.ctx, call)
		if r.ctx.Err() != nil {
			r.abandonCalls(calls[index:])
			r.finishAborted()
			return false
		}
		r.session.Messages = append(r.session.Messages, Message{Role: RoleTool, ToolCallID: call.ID, Text: outcome.Text})
		r.emit(Event{Kind: EventToolEnd, Tool: call.Name, CallID: call.ID, IsError: outcome.IsError})
	}
	return true
}

func (r *turnRun) abandonCalls(calls []ToolCall) {
	for _, call := range calls {
		r.session.Messages = append(r.session.Messages, Message{Role: RoleTool, ToolCallID: call.ID, Text: abortedByCallerText})
	}
}

// callAssembler joins streamed tool-call pieces by index and returns the calls in index order.
type callAssembler struct {
	byIndex map[int]*ToolCall
}

func newCallAssembler() *callAssembler {
	return &callAssembler{byIndex: map[int]*ToolCall{}}
}

func (a *callAssembler) add(delta ToolCallDelta) {
	call, ok := a.byIndex[delta.Index]
	if !ok {
		call = &ToolCall{ID: fmt.Sprintf("call-%d", delta.Index)}
		a.byIndex[delta.Index] = call
	}
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Name != "" {
		call.Name = delta.Name
	}
	call.Arguments += delta.ArgumentsDelta
}

func (a *callAssembler) list() []ToolCall {
	calls := make([]ToolCall, 0, len(a.byIndex))
	for _, index := range slices.Sorted(maps.Keys(a.byIndex)) {
		calls = append(calls, *a.byIndex[index])
	}
	return calls
}
