package agent

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// abandonedResult is what the model reads for a call the turn stopped before it ran, so the transcript stays valid.
const abandonedResult = "Tool call was not run because the turn was cancelled."

// callResult is how one call ended: its status, the text the model reads and the line the transcript shows.
type callResult struct {
	status  CallStatus
	result  string
	outcome string
}

// runCalls executes calls in order, one tool message each. It reports false when the turn was cancelled; the calls
// after the cancelled one are then reported as cancelled, so every call ends with one tool_end event.
func (r *turnRun) runCalls(calls []backend.ToolCall) bool {
	for index, call := range calls {
		if r.pastDeadline() || r.ctx.Err() != nil {
			r.abandon(calls[index:])
			r.finish(r.endEvent())
			return false
		}
		outcome := r.runCall(call)
		if outcome.status == CallCancelled {
			r.abandon(calls[index+1:])
			r.finish(r.endEvent())
			return false
		}
		r.messages = append(r.messages, toolMessage(call.ID, outcome.result))
	}
	return true
}

// runCall reports one call from start to end: tool_start, any approval, then tool_end.
func (r *turnRun) runCall(call backend.ToolCall) callResult {
	tool, known := r.setup.Tools.Lookup(call.Function.Name)
	info := callInfoOf(call, tool, known)
	r.emit(Event{Kind: EventToolStart, Call: info})
	outcome := r.decideAndRun(call, tool, known, info)
	r.emit(Event{Kind: EventToolEnd, Call: info, Status: outcome.status, Result: outcome.result, Outcome: outcome.outcome})
	return outcome
}

// abandon reports the calls a cancelled turn did not run, and answers each one for the transcript.
func (r *turnRun) abandon(calls []backend.ToolCall) {
	for _, call := range calls {
		tool, known := r.setup.Tools.Lookup(call.Function.Name)
		info := callInfoOf(call, tool, known)
		r.emit(Event{Kind: EventToolStart, Call: info})
		r.emit(Event{Kind: EventToolEnd, Call: info, Status: CallCancelled, Result: abandonedResult, Outcome: "Stopped before it ran"})
	}
}

// decideAndRun checks a call in the order the host promises: the tool must exist, its arguments must match the schema,
// the policy must allow it, the user must allow it when the tool asks, and only then does it run.
func (r *turnRun) decideAndRun(call backend.ToolCall, tool Tool, known bool, info CallInfo) callResult {
	if !known {
		return callResult{status: CallFailed, result: call.Function.Name + " is not a registered tool", outcome: "Unknown tool"}
	}
	arguments := normalizedArguments(call.Function.Arguments)
	if !isJSONObject(arguments) {
		return callResult{status: CallFailed, result: "tool arguments must be a JSON object", outcome: "Invalid arguments"}
	}
	if err := ValidateArguments(tool.Schema, json.RawMessage(arguments)); err != nil {
		return callResult{status: CallFailed, result: err.Error(), outcome: "Invalid arguments"}
	}
	if decision := r.setup.Policy.Decide(call, tool); !decision.Allow {
		return callResult{status: CallRefused, result: "tool call refused: " + decision.Reason, outcome: "Refused: " + decision.Reason}
	}
	if tool.Confirm {
		asked := r.setup.Clock.Now()
		allowed, cancelled := r.await(r.ctx, call.ID, info)
		r.waited.Add(int64(r.setup.Clock.Now().Sub(asked)))
		switch {
		case cancelled:
			return callResult{status: CallCancelled, result: abandonedResult, outcome: "Stopped before it ran"}
		case !allowed:
			return callResult{status: CallRefused, result: "The user did not allow this tool call.", outcome: "Refused by you"}
		}
	}
	return r.run(tool, json.RawMessage(arguments), info)
}

// await asks the user to allow a call and waits for the answer or the end of ctx, which is the turn's context or one
// derived from it.
func (r *turnRun) await(ctx context.Context, callID string, info CallInfo) (allowed, cancelled bool) {
	answer := make(chan bool, 1)
	r.turn.mu.Lock()
	r.turn.waiting[callID] = answer
	r.turn.mu.Unlock()
	r.emit(Event{Kind: EventApproval, Call: info})
	select {
	case allowed := <-answer:
		return allowed, false
	case <-ctx.Done():
		r.turn.mu.Lock()
		delete(r.turn.waiting, callID)
		r.turn.mu.Unlock()
		return false, true
	}
}

// toolMessage is the transcript entry that answers a call.
func toolMessage(callID, text string) backend.Message {
	return backend.Message{Role: backend.RoleTool, ToolCallID: callID, Content: backend.TextContent(text)}
}

// callInfoOf describes a call for the events: its label, and the summary of its arguments when the tool gives one.
func callInfoOf(call backend.ToolCall, tool Tool, known bool) CallInfo {
	info := CallInfo{ID: call.ID, Name: call.Function.Name, Label: call.Function.Name}
	if !known {
		return info
	}
	info.Label = tool.Label
	arguments := normalizedArguments(call.Function.Arguments)
	if tool.Summarize != nil && isJSONObject(arguments) {
		info.Input = tool.Summarize(json.RawMessage(arguments))
	}
	return info
}
