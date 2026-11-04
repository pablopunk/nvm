package agent

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// ModelChunk is one streamed piece of a model turn: text, a piece of a tool call, or token usage.
type ModelChunk struct {
	Text      string
	ToolDelta *backend.ToolCallDelta
	Usage     *Usage
}

// ModelStream yields the chunks of one model turn. Next returns io.EOF after the last chunk of a complete reply, and
// any other error for a reply that broke. Close releases the stream; the loop closes every stream it opens.
type ModelStream interface {
	Next() (ModelChunk, error)
	Close() error
}

// Model opens one streamed model turn over messages, offering tools (a JSON array, or nil for none). Implementations
// stop when ctx is cancelled.
type Model interface {
	Stream(ctx context.Context, messages []backend.Message, tools json.RawMessage) (ModelStream, error)
}

// Usage is the token count of model turns.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

func (u *Usage) add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
}
