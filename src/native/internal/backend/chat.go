package backend

import (
	"context"
	"encoding/json"
	"fmt"
)

// Message roles the chat route accepts.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// AutoModel is the model sentinel for chat requests. The backend routes the request to the active model for
// the caller's plan and ignores the name, so clients never hardcode a provider model ID.
const AutoModel = "auto"

// ToolCall is a tool invocation the assistant made, sent back in the history with its result.
type ToolCall struct {
	// ID links the call to the tool message that answers it.
	ID string `json:"id"`
	// Type is always "function" for the chat route.
	Type string `json:"type"`
	// Function names the tool and carries its JSON arguments.
	Function ToolFunction `json:"function"`
}

// ToolFunction is the name and JSON-encoded arguments of a tool call.
type ToolFunction struct {
	// Name is the tool name.
	Name string `json:"name"`
	// Arguments is the JSON object text the model produced.
	Arguments string `json:"arguments"`
}

// Message is one turn of the conversation in the OpenAI chat format.
type Message struct {
	// Role is one of the Role constants.
	Role string `json:"role"`
	// Content is a JSON string or JSON array of content parts. Use TextContent for plain text. Nil omits it,
	// which is valid for an assistant message that only carries tool calls.
	Content json.RawMessage `json:"content,omitempty"`
	// Name optionally names the author.
	Name string `json:"name,omitempty"`
	// ToolCallID answers a tool call; set it on tool messages.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolCalls lists the calls an assistant message made.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// RoleHeader carries the model tier the backend routes the request to. The client names a tier, never a model.
const RoleHeader = "X-Nevermind-AI-Model"

// Model tiers the backend accepts in RoleHeader and in ActiveModelQuery.Role.
const (
	RoleFast  = "fast"
	RoleSmart = "smart"
)

// ChatRequest is one streamed chat completion.
type ChatRequest struct {
	// Model is the model name. Empty means AutoModel.
	Model string
	// Role is the model tier, RoleFast or RoleSmart. Empty leaves the backend default, which is smart.
	Role string
	// Messages is the conversation, oldest first. At least one is required.
	Messages []Message
	// Tools is an optional JSON array of tool definitions, passed through unchanged.
	Tools json.RawMessage
	// ToolChoice is an optional JSON value for tool_choice, passed through unchanged.
	ToolChoice json.RawMessage
	// MaxTokens caps the completion length. Zero leaves the server default in place.
	MaxTokens int
}

// chatWire is the request body the route receives. Stream is always true.
type chatWire struct {
	Model      string          `json:"model"`
	Messages   []Message       `json:"messages"`
	Tools      json.RawMessage `json:"tools,omitempty"`
	ToolChoice json.RawMessage `json:"tool_choice,omitempty"`
	MaxTokens  int             `json:"max_tokens,omitempty"`
	Stream     bool            `json:"stream"`
}

// TextContent returns the JSON string encoding of text, for Message.Content.
func TextContent(text string) json.RawMessage {
	data, err := json.Marshal(text)
	if err != nil {
		return nil
	}
	return data
}

// ChatStream sends req and returns the open event stream. The call is never retried: a retried stream could
// bill twice and deliver text twice. Cancel ctx to stop the stream. The backend routes the request to the
// account's active model, which must speak openai-completions; check ActiveModel's SpeaksChatCompletions first.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	body, err := encodeChat(req)
	if err != nil {
		return nil, err
	}
	header := map[string]string{"Accept": "text/event-stream"}
	if req.Role != "" {
		if !isRole(req.Role) {
			return nil, &Error{Kind: KindRejected, Message: fmt.Sprintf("unknown model role %q", req.Role)}
		}
		header[RoleHeader] = req.Role
	}
	return c.openStream(ctx, requestSpec{
		method:      "POST",
		path:        pathChatCompletions,
		body:        body,
		header:      header,
		auth:        authRequired,
		idleTimeout: StreamIdleTimeout,
	})
}

// isRole reports whether role is a model tier the backend routes.
func isRole(role string) bool {
	return role == RoleFast || role == RoleSmart
}

// encodeChat validates req and returns its JSON body. Validation failures are local and never reach the
// backend.
func encodeChat(req ChatRequest) ([]byte, error) {
	if len(req.Messages) == 0 {
		return nil, &Error{Kind: KindRejected, Message: "a chat request needs at least one message"}
	}
	for index, message := range req.Messages {
		if !isChatRole(message.Role) {
			return nil, &Error{Kind: KindRejected, Message: fmt.Sprintf("message %d has unknown role %q", index, message.Role)}
		}
	}
	if req.Tools != nil && !json.Valid(req.Tools) {
		return nil, &Error{Kind: KindRejected, Message: "tools is not valid JSON"}
	}
	if req.ToolChoice != nil && !json.Valid(req.ToolChoice) {
		return nil, &Error{Kind: KindRejected, Message: "tool_choice is not valid JSON"}
	}
	model := req.Model
	if model == "" {
		model = AutoModel
	}
	body, err := json.Marshal(chatWire{
		Model:      model,
		Messages:   req.Messages,
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
		MaxTokens:  req.MaxTokens,
		Stream:     true,
	})
	if err != nil {
		return nil, &Error{Kind: KindRejected, Message: "could not encode the chat request", Cause: err}
	}
	return body, nil
}

// isChatRole reports whether role is one the chat route accepts.
func isChatRole(role string) bool {
	switch role {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return true
	default:
		return false
	}
}
