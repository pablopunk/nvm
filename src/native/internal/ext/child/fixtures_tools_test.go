package child

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// A real child serves a tool call: it announces the tool entry points, and CallAITool gets the call's name and JSON
// arguments as the host sent them.
func TestToolsFixtureAnswersAToolCallThroughTheChild(t *testing.T) {
	call := api.AIToolCall{Name: "echo", Args: json.RawMessage(`{"text":"hello"}`)}
	ready, responses := exchange(t, "acme.tools", protocol.Request{ID: 3, Method: protocol.MethodAITool, Invocation: protocol.AIToolInvocation(call)})
	if !slices.Contains(ready.Methods, protocol.MethodAITool) || !slices.Contains(ready.Methods, protocol.MethodDescribeAITool) {
		t.Fatalf("ready methods = %v; want the tool entry points", ready.Methods)
	}
	response := responseWithID(t, responses, 3)
	if response.Failure != nil || string(response.Data) != `{"text":"hello"}` {
		t.Fatalf("response = %+v (%s)", response, response.Data)
	}
}

// Without a host the fixture's describe fails on its storage read, so a call the host cannot describe is refused.
func TestToolsFixtureDescribeFailsWithoutAHost(t *testing.T) {
	call := api.AIToolCall{Name: "reset-count"}
	_, responses := exchange(t, "acme.tools", protocol.Request{ID: 4, Method: protocol.MethodDescribeAITool, Invocation: protocol.AIToolInvocation(call)})
	if response := responseWithID(t, responses, 4); response.Failure == nil {
		t.Fatalf("response = %+v; want the storage failure", response)
	}
}

func TestToolsFixtureListShowsTheEmptyStateWithoutAHost(t *testing.T) {
	result := runHostlessFixtureCommand(t, "acme.tools", "tools", nil)
	if result.View == nil || len(result.View.Items) != 0 || result.View.EmptyView == nil {
		t.Fatalf("view = %+v; want an empty list with an empty state", result.View)
	}
}

func responseWithID(t *testing.T, responses []protocol.Response, id uint64) protocol.Response {
	t.Helper()
	for _, response := range responses {
		if response.ID == id {
			return response
		}
	}
	t.Fatalf("no response %d in %+v", id, responses)
	return protocol.Response{}
}
