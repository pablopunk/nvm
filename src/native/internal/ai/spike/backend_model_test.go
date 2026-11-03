package spike

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// The adapter's stream loop needs a live *backend.Client, which needs an HTTP port this package may not import. The
// tests therefore cover the pure event mapping and the wire encoding; the backend package's own fake-HTTP tests cover
// ChatStream itself.

func TestMapBackendEventCoversEveryStreamKind(t *testing.T) {
	cases := []struct {
		name     string
		event    backend.Event
		wantSkip bool
		check    func(t *testing.T, chunk ModelChunk, err error)
	}{
		{
			name:  "content becomes text",
			event: backend.Event{Type: backend.EventContent, Text: "hi"},
			check: func(t *testing.T, chunk ModelChunk, err error) {
				if err != nil || chunk.Text != "hi" {
					t.Fatalf("chunk = %+v, err = %v", chunk, err)
				}
			},
		},
		{
			name: "tool call delta keeps index, id, name and arguments",
			event: backend.Event{Type: backend.EventToolCallDelta, ToolCall: backend.ToolCallDelta{
				Index: 2, ID: "c9", Name: "read", ArgumentsDelta: `{"path":`,
			}},
			check: func(t *testing.T, chunk ModelChunk, err error) {
				want := ToolCallDelta{Index: 2, ID: "c9", Name: "read", ArgumentsDelta: `{"path":`}
				if err != nil || chunk.ToolDelta == nil || *chunk.ToolDelta != want {
					t.Fatalf("tool delta = %+v, err = %v", chunk.ToolDelta, err)
				}
			},
		},
		{
			name:  "usage is carried",
			event: backend.Event{Type: backend.EventUsage, Usage: backend.Usage{InputTokens: 10, OutputTokens: 4}},
			check: func(t *testing.T, chunk ModelChunk, err error) {
				if err != nil || chunk.Usage == nil || *chunk.Usage != (Usage{InputTokens: 10, OutputTokens: 4}) {
					t.Fatalf("usage = %+v, err = %v", chunk.Usage, err)
				}
			},
		},
		{
			name:  "done carries the finish reason",
			event: backend.Event{Type: backend.EventDone, FinishReason: "tool_calls"},
			check: func(t *testing.T, chunk ModelChunk, err error) {
				if err != nil || chunk.FinishReason != "tool_calls" {
					t.Fatalf("finish = %+v, err = %v", chunk, err)
				}
			},
		},
		{
			name:     "reasoning is skipped",
			event:    backend.Event{Type: backend.EventReasoning, Text: "thinking"},
			wantSkip: true,
			check: func(t *testing.T, _ ModelChunk, err error) {
				if err != nil {
					t.Fatalf("reasoning must not be an error: %v", err)
				}
			},
		},
		{
			name: "backend error keeps its code",
			event: backend.Event{Type: backend.EventError, Err: &backend.Error{
				Kind: backend.KindRejected, Status: 409, Code: "model_route_changed", Message: "route changed",
			}},
			check: func(t *testing.T, _ ModelChunk, err error) {
				var modelErr *ModelError
				if !errors.As(err, &modelErr) || modelErr.Code != "model_route_changed" {
					t.Fatalf("error = %v, want a ModelError with the backend code", err)
				}
			},
		},
		{
			name:  "backend error without a code falls back to its kind",
			event: backend.Event{Type: backend.EventError, Err: &backend.Error{Kind: backend.KindRateLimited}},
			check: func(t *testing.T, _ ModelChunk, err error) {
				var modelErr *ModelError
				if !errors.As(err, &modelErr) || modelErr.Code != string(backend.KindRateLimited) {
					t.Fatalf("error = %v, want the rate_limited kind", err)
				}
			},
		},
		{
			name:  "an error event with no payload is a bad response",
			event: backend.Event{Type: backend.EventError},
			check: func(t *testing.T, _ ModelChunk, err error) {
				var modelErr *ModelError
				if !errors.As(err, &modelErr) || modelErr.Code != "bad_response" {
					t.Fatalf("error = %v, want bad_response", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chunk, skip, err := mapBackendEvent(tc.event)
			if skip != tc.wantSkip {
				t.Fatalf("skip = %v, want %v", skip, tc.wantSkip)
			}
			tc.check(t, chunk, err)
		})
	}
}

func TestBackendMessagesEncodeToolRoundTripInChatShape(t *testing.T) {
	messages := []Message{
		{Role: RoleUser, Text: "read notes"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "c1", Name: "read", Arguments: `{"path":"notes.md"}`}}},
		{Role: RoleTool, ToolCallID: "c1", Text: "hello"},
	}

	data, err := json.Marshal(backendMessages(messages))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(data)
	for _, fragment := range []string{
		`{"role":"user","content":"read notes"}`,
		`{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{\"path\":\"notes.md\"}"}}]}`,
		`{"role":"tool","content":"hello","tool_call_id":"c1"}`,
	} {
		if !strings.Contains(wire, fragment) {
			t.Fatalf("wire JSON lacks %s\nin: %s", fragment, wire)
		}
	}
}

func TestEncodeBackendToolsDefaultsTheSchemaAndOmitsEmptyLists(t *testing.T) {
	if data, err := encodeBackendTools(nil); err != nil || data != nil {
		t.Fatalf("no tools must encode to nothing, got %s, %v", data, err)
	}
	data, err := encodeBackendTools([]ToolSpec{{Name: "ls", Description: "list"}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := `[{"type":"function","function":{"name":"ls","description":"list","parameters":{"type":"object","properties":{}}}}]`
	if string(data) != want {
		t.Fatalf("tools = %s\nwant   %s", data, want)
	}
}
