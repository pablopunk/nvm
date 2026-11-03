package backend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// userMessages is a one-message conversation for tests that only need a valid request.
func userMessages(text string) []Message {
	return []Message{{Role: RoleUser, Content: TextContent(text)}}
}

// drainStream reads every event until io.EOF and closes the stream. It fails the test when the stream never ends.
func drainStream(t *testing.T, stream *Stream) []Event {
	t.Helper()
	defer func() { _ = stream.Close() }()
	var events []Event
	for range 100000 {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			return events
		}
		if err != nil {
			t.Fatalf("Next returned a Go error %v; failures must arrive as EventError", err)
		}
		events = append(events, event)
	}
	t.Fatal("stream never reached io.EOF")
	return nil
}

// contentOf joins the content pieces of a stream.
func contentOf(events []Event) string {
	var builder strings.Builder
	for _, event := range events {
		if event.Type == EventContent {
			builder.WriteString(event.Text)
		}
	}
	return builder.String()
}

// TestHealthyChatStreamDeliversTheReplyAndDone checks the fixture's reply arrives whole, in order, followed by
// exactly one Done and then io.EOF.
func TestHealthyChatStreamDeliversTheReplyAndDone(t *testing.T) {
	h := newHarness(t)
	h.withBackend(t)
	stream, err := h.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	events := drainStream(t, stream)
	if got := contentOf(events); got != fixtures.FixtureReply {
		t.Fatalf("streamed text = %q, want the fixture reply", got)
	}
	if last := events[len(events)-1]; last.Type != EventDone {
		t.Fatalf("last event = %+v, want EventDone", last)
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after the end = %v, want io.EOF", err)
	}
}

// TestChatRequestSendsTheStreamingContract checks the request asks for the event stream, names the auto model
// sentinel, and carries the messages as the route reads them.
func TestChatRequestSendsTheStreamingContract(t *testing.T) {
	h := newHarness(t)
	h.withBackend(t)
	stream, err := h.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi"), MaxTokens: 20})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	drainStream(t, stream)
	request := h.http.Requests()[0]
	if got := request.Header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want text/event-stream", got)
	}
	var body map[string]any
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("request body is not JSON: %v", err)
	}
	if body["model"] != AutoModel || body["stream"] != true || body["max_tokens"] != float64(20) {
		t.Fatalf("body = %v, want model auto, stream true, max_tokens 20", body)
	}
}

// TestChatStreamOpenFailureIsReturnedBeforeAnyEvent checks a refused request fails the call itself, so the caller
// never has to read a stream that never started.
func TestChatStreamOpenFailureIsReturnedBeforeAnyEvent(t *testing.T) {
	h := newHarness(t)
	backend := h.withBackend(t)
	backend.SetMode(fixtures.ModeUnauthorized)
	stream, err := h.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if stream != nil || !IsKind(err, KindUnauthorized) {
		t.Fatalf("ChatStream = (%v, %v), want no stream and KindUnauthorized", stream, err)
	}
}

// TestMalformedStreamEndsWithAnErrorNotACompletion checks a broken frame mid-stream keeps the text received so
// far, reports the stream as broken, and never emits Done.
func TestMalformedStreamEndsWithAnErrorNotACompletion(t *testing.T) {
	h := newHarness(t)
	backend := h.withBackend(t)
	backend.SetMode(fixtures.ModeMalformedSSE)
	stream, err := h.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	events := drainStream(t, stream)
	assertTerminalError(t, events, KindBadResponse)
	if got := contentOf(events); got != fixtures.FixtureReply {
		t.Fatalf("text before the break = %q, want the whole reply", got)
	}
}

// TestMidStreamErrorEventEndsTheStream checks an upstream failure after the first token surfaces as a retryable
// server error with the backend's message.
func TestMidStreamErrorEventEndsTheStream(t *testing.T) {
	h := newHarness(t)
	backend := h.withBackend(t)
	backend.SetMode(fixtures.ModeMidStreamError)
	stream, err := h.client.ChatStream(context.Background(), ChatRequest{Messages: userMessages("hi")})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	events := drainStream(t, stream)
	assertTerminalError(t, events, KindServer)
	terminal := events[len(events)-1].Err
	if !terminal.Retryable || !strings.Contains(terminal.Message, "Upstream connection closed mid-stream") {
		t.Fatalf("terminal error = %+v, want retryable with the upstream message", terminal)
	}
}

// assertTerminalError checks the last event is an EventError of kind and that no EventDone was emitted.
func assertTerminalError(t *testing.T, events []Event, kind Kind) {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("stream produced no events")
	}
	last := events[len(events)-1]
	if last.Type != EventError || last.Err == nil || last.Err.Kind != kind {
		t.Fatalf("last event = %+v, want EventError of kind %s", last, kind)
	}
	for _, event := range events {
		if event.Type == EventDone {
			t.Fatal("a broken stream emitted EventDone")
		}
	}
}

// TestEncodeChatValidatesBeforeSending checks requests the backend would refuse are refused locally, with the
// reason, and that valid requests default the model to the auto sentinel.
func TestEncodeChatValidatesBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		request ChatRequest
		wantErr bool
	}{
		{name: "no messages", request: ChatRequest{}, wantErr: true},
		{name: "unknown role", request: ChatRequest{Messages: []Message{{Role: "developer"}}}, wantErr: true},
		{name: "invalid tools JSON", request: ChatRequest{Messages: userMessages("x"), Tools: json.RawMessage(`[`)}, wantErr: true},
		{name: "invalid tool_choice JSON", request: ChatRequest{Messages: userMessages("x"), ToolChoice: json.RawMessage(`{`)}, wantErr: true},
		{name: "tool role with tool call id", request: ChatRequest{Messages: []Message{{Role: RoleTool, ToolCallID: "call_1", Content: TextContent("ok")}}}},
		{name: "valid", request: ChatRequest{Messages: userMessages("x")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encodeChat(tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("encodeChat error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
	body, err := encodeChat(ChatRequest{Messages: userMessages("x"), Tools: json.RawMessage(`[{"type":"function"}]`)})
	if err != nil {
		t.Fatalf("encodeChat: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if wire["model"] != AutoModel || wire["stream"] != true {
		t.Fatalf("body = %s, want model auto and stream true", body)
	}
	if _, ok := wire["tools"].([]any); !ok {
		t.Fatalf("tools were not passed through: %s", body)
	}
	if _, ok := wire["max_tokens"]; ok {
		t.Fatalf("max_tokens sent when zero: %s", body)
	}
}
