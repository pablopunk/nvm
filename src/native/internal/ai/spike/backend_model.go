package spike

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// BackendModel adapts the backend chat client to Model. It sends to /api/v1/chat/completions, so the account's active
// route must speak openai-completions; ChatStream reports a local error for other routes before anything is sent.
type BackendModel struct {
	Client *backend.Client
	Model  string
}

// Stream opens one streamed completion with the request's messages and tools.
func (m BackendModel) Stream(ctx context.Context, request ModelRequest) (ModelStream, error) {
	tools, err := encodeBackendTools(request.Tools)
	if err != nil {
		return nil, err
	}
	stream, err := m.Client.ChatStream(ctx, backend.ChatRequest{
		Model:    m.Model,
		Messages: backendMessages(request.Messages),
		Tools:    tools,
	})
	if err != nil {
		return nil, err
	}
	return &backendStream{stream: stream}, nil
}

type backendStream struct {
	stream *backend.Stream
}

// Next maps backend events to model chunks. A backend error event becomes a ModelError carrying the backend's code.
func (s *backendStream) Next() (ModelChunk, error) {
	for {
		event, err := s.stream.Next()
		if err != nil {
			return ModelChunk{}, err
		}
		chunk, skip, mapErr := mapBackendEvent(event)
		if mapErr != nil {
			return ModelChunk{}, mapErr
		}
		if !skip {
			return chunk, nil
		}
	}
}

func (s *backendStream) Close() error {
	return s.stream.Close()
}

// mapBackendEvent converts one backend event. Reasoning text is skipped: the spike's loop does not show it.
func mapBackendEvent(event backend.Event) (ModelChunk, bool, error) {
	switch event.Type {
	case backend.EventContent:
		return ModelChunk{Text: event.Text}, false, nil
	case backend.EventToolCallDelta:
		delta := event.ToolCall
		return ModelChunk{ToolDelta: &ToolCallDelta{
			Index:          delta.Index,
			ID:             delta.ID,
			Name:           delta.Name,
			ArgumentsDelta: delta.ArgumentsDelta,
		}}, false, nil
	case backend.EventUsage:
		return ModelChunk{Usage: &Usage{
			InputTokens:  event.Usage.InputTokens,
			OutputTokens: event.Usage.OutputTokens,
		}}, false, nil
	case backend.EventDone:
		return ModelChunk{FinishReason: event.FinishReason}, false, nil
	case backend.EventError:
		return ModelChunk{}, false, backendModelError(event.Err)
	default:
		return ModelChunk{}, true, nil
	}
}

func backendModelError(failure *backend.Error) error {
	if failure == nil {
		return &ModelError{Code: "bad_response", Message: "the backend ended the stream with an unknown error"}
	}
	code := failure.Code
	if code == "" {
		code = string(failure.Kind)
	}
	return &ModelError{Code: code, Message: failure.Message}
}

// backendMessages converts transcript messages to the chat wire shape.
func backendMessages(messages []Message) []backend.Message {
	wire := make([]backend.Message, 0, len(messages))
	for _, message := range messages {
		entry := backend.Message{Role: string(message.Role), ToolCallID: message.ToolCallID}
		if message.Text != "" || message.Role != RoleAssistant || len(message.ToolCalls) == 0 {
			entry.Content = backend.TextContent(message.Text)
		}
		for _, call := range message.ToolCalls {
			entry.ToolCalls = append(entry.ToolCalls, backend.ToolCall{
				ID:       call.ID,
				Type:     "function",
				Function: backend.ToolFunction{Name: call.Name, Arguments: call.Arguments},
			})
		}
		wire = append(wire, entry)
	}
	return wire
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// encodeBackendTools returns the JSON tools array for the chat request, or nil when there are no tools.
func encodeBackendTools(specs []ToolSpec) (json.RawMessage, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	tools := make([]wireTool, 0, len(specs))
	for _, spec := range specs {
		parameters := spec.Schema
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		tools = append(tools, wireTool{
			Type:     "function",
			Function: wireFunction{Name: spec.Name, Description: spec.Description, Parameters: parameters},
		})
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return nil, fmt.Errorf("encode tools: %w", err)
	}
	return data, nil
}
