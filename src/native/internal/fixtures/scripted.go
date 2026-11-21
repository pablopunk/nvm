package fixtures

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// ScriptedReply is one recorded answer to a chat request. A reply with Status zero or 200 streams Frames; any other
// status answers with the backend's error body, whose error type is ErrorType.
type ScriptedReply struct {
	Frames    []string
	Status    int
	ErrorType string
}

// ToolCall is one tool call a recorded model turn makes. Arguments are sent in pieces, as the model streams them.
type ToolCall struct {
	ID        string
	Name      string
	Arguments []string
}

// ScriptedBackend is a backend for tests of the agent loop. It answers the active-model route with ActiveModelBody and
// each chat request with the next recorded reply. A request past the script fails with 500, so a loop that asks too
// often shows up at once. Chat request bodies are recorded in order.
type ScriptedBackend struct {
	mu       sync.Mutex
	replies  []ScriptedReply
	requests []string
}

// NewScriptedBackend returns a backend that answers chat requests with replies, in order.
func NewScriptedBackend(replies ...ScriptedReply) *ScriptedBackend {
	return &ScriptedBackend{replies: replies}
}

// Requests returns the bodies of the chat requests the backend received, oldest first.
func (b *ScriptedBackend) Requests() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.requests...)
}

// ServeHTTP answers one backend request.
func (b *ScriptedBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == ActiveModelPath {
		writeBody(w, http.StatusOK, "application/json", ActiveModelBody)
		return
	}
	body, _ := io.ReadAll(r.Body)
	b.mu.Lock()
	b.requests = append(b.requests, string(body))
	if len(b.requests) > len(b.replies) {
		b.mu.Unlock()
		writeBody(w, http.StatusInternalServerError, "application/json", `{"error":{"type":"script_exhausted","message":"no recorded reply left"}}`)
		return
	}
	reply := b.replies[len(b.requests)-1]
	b.mu.Unlock()
	if reply.Status != 0 && reply.Status != http.StatusOK {
		writeBody(w, reply.Status, "application/json", errorBody(reply.ErrorType))
		return
	}
	writeBody(w, http.StatusOK, "text/event-stream", strings.Join(reply.Frames, ""))
}

// errorBody is the backend's error envelope for a type.
func errorBody(errorType string) string {
	data, err := json.Marshal(map[string]any{"error": map[string]string{"type": errorType, "message": "recorded failure"}})
	if err != nil {
		panic("fixtures: encoding an error body failed: " + err.Error())
	}
	return string(data)
}

// TextReply is a recorded reply that streams text and ends with the end-of-stream marker.
func TextReply(text string) ScriptedReply {
	return ScriptedReply{Frames: append(StreamFrames(text, 16), DoneFrame)}
}

// ToolCallReply is a recorded reply that streams text, then the tool calls, and ends with the end-of-stream marker.
func ToolCallReply(text string, calls ...ToolCall) ScriptedReply {
	var frames []string
	if text != "" {
		frames = StreamFrames(text, 16)
	}
	for _, call := range calls {
		frames = append(frames, toolCallFrame(call.ID, call.Name, ""))
		for _, piece := range call.Arguments {
			frames = append(frames, toolArgumentFrame(piece))
		}
	}
	return ScriptedReply{Frames: append(frames, DoneFrame)}
}

// toolCallFrame encodes the first piece of a tool call, with its id and name, as an SSE frame.
func toolCallFrame(id, name, arguments string) string {
	return chunkFrame(fmt.Sprintf(`{"tool_calls":[{"index":0,"id":%q,"function":{"name":%q,"arguments":%q}}]}`, id, name, arguments))
}

// toolArgumentFrame encodes one streamed piece of arguments as an SSE frame.
func toolArgumentFrame(piece string) string {
	return chunkFrame(fmt.Sprintf(`{"tool_calls":[{"index":0,"function":{"arguments":%q}}]}`, piece))
}

// chunkFrame wraps a delta object in a chat completion chunk.
func chunkFrame(delta string) string {
	return `data: {"object":"chat.completion.chunk","choices":[{"index":0,"delta":` + delta + `}]}` + "\n\n"
}
