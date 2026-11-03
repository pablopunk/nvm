package spike

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

// ToolSpec describes a tool to the model. Schema is a JSON Schema object.
type ToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// ModelRequest is what the loop sends for one model turn.
type ModelRequest struct {
	Messages []Message
	Tools    []ToolSpec
}

// ToolCallDelta is one streamed piece of a tool call. Pieces with the same Index belong to one call.
type ToolCallDelta struct {
	Index          int
	ID             string
	Name           string
	ArgumentsDelta string
}

// Usage is token accounting for one or more model turns.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

func (u *Usage) add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
}

// ModelChunk is one streamed piece of an assistant turn.
type ModelChunk struct {
	Text         string
	ToolDelta    *ToolCallDelta
	Usage        *Usage
	FinishReason string
}

// ModelStream yields the chunks of one turn. Next returns io.EOF after the last chunk. Close releases the stream and
// the loop calls it exactly once per turn, including on cancellation.
type ModelStream interface {
	Next() (ModelChunk, error)
	Close() error
}

// Model opens one streamed assistant turn. Implementations must stop when ctx is cancelled.
type Model interface {
	Stream(ctx context.Context, req ModelRequest) (ModelStream, error)
}

// ModelError is a failure the model reports with a machine-readable Code, such as the backend error type.
type ModelError struct {
	Code    string
	Message string
}

func (e *ModelError) Error() string {
	return e.Code + ": " + e.Message
}

func closeStream(stream ModelStream) {
	_ = stream.Close()
}

func isStreamEnd(err error) bool {
	return errors.Is(err, io.EOF)
}
