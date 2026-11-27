package agent

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Waiver lets a confirming call run without its question when the host already holds the user's word for exactly that
// call, such as the one launch the user's current message asks for. A waiver is spent by the call it lets through.
type Waiver interface {
	Waive(call backend.ToolCall, tool Tool) bool
}

// declinedCalls are the exact calls the user refused in a turn. A refused call is not asked again in the turn.
type declinedCalls struct {
	mu    sync.Mutex
	calls map[string]bool
}

func (d *declinedCalls) remember(call backend.ToolCall) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.calls == nil {
		d.calls = map[string]bool{}
	}
	d.calls[exactCall(call)] = true
}

func (d *declinedCalls) has(call backend.ToolCall) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls[exactCall(call)]
}

// exactCall names a call by its tool and canonical arguments, so one answer covers only that operation and resource.
func exactCall(call backend.ToolCall) string {
	return call.Function.Name + "|" + CanonicalArguments(json.RawMessage(call.Function.Arguments))
}

// CanonicalArguments is a call's arguments with their keys sorted and their spacing removed, so two spellings of one
// call name one resource. Arguments that are not JSON are returned as they are.
func CanonicalArguments(args json.RawMessage) string {
	trimmed := bytes.TrimSpace(args)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "{}"
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return string(trimmed)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return string(trimmed)
	}
	return string(canonical)
}

// confirm settles a confirming call before it runs. It reports whether the call may run, and the result to report
// when it may not: a call the user refused earlier in the turn fails without asking, a waived call runs without asking,
// and every other call waits for the user.
func (r *turnRun) confirm(call backend.ToolCall, tool Tool, info CallInfo) (callResult, bool) {
	if r.declined.has(call) {
		return callResult{status: CallRefused, result: "The user already refused this exact tool call in this turn, so it was not asked again and did not run. Do not try it again.",
			outcome: "Refused by you earlier"}, false
	}
	if r.setup.Waiver != nil && r.setup.Waiver.Waive(call, tool) {
		return callResult{}, true
	}
	asked := r.setup.Clock.Now()
	allowed, cancelled := r.await(r.ctx, call.ID, info)
	r.waited.Add(int64(r.setup.Clock.Now().Sub(asked)))
	switch {
	case cancelled:
		return callResult{status: CallCancelled, result: abandonedResult, outcome: "Stopped before it ran"}, false
	case !allowed:
		r.declined.remember(call)
		return callResult{status: CallRefused, result: "The user did not allow this tool call.", outcome: "Refused by you"}, false
	}
	return callResult{}, true
}
