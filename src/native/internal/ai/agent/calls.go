package agent

import (
	"fmt"
	"maps"
	"slices"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// callAssembler joins streamed tool-call pieces by index and returns the calls in index order. A call that arrives
// without an id gets a stable one, so its result can still be matched to it.
type callAssembler struct {
	byIndex map[int]*backend.ToolCall
}

func newCallAssembler() *callAssembler {
	return &callAssembler{byIndex: map[int]*backend.ToolCall{}}
}

// named reports whether the call at index already has its tool name.
func (a *callAssembler) named(index int) bool {
	call, ok := a.byIndex[index]
	return ok && call.Function.Name != ""
}

func (a *callAssembler) add(delta backend.ToolCallDelta) {
	call, ok := a.byIndex[delta.Index]
	if !ok {
		call = &backend.ToolCall{Type: "function", ID: fmt.Sprintf("call_%d", delta.Index)}
		a.byIndex[delta.Index] = call
	}
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Name != "" {
		call.Function.Name = delta.Name
	}
	call.Function.Arguments += delta.ArgumentsDelta
}

func (a *callAssembler) list() []backend.ToolCall {
	calls := make([]backend.ToolCall, 0, len(a.byIndex))
	for _, index := range slices.Sorted(maps.Keys(a.byIndex)) {
		calls = append(calls, *a.byIndex[index])
	}
	return calls
}
