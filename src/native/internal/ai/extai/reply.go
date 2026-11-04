package extai

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
)

// Reply is a reply that streams. Next returns its text piece by piece; Abort stops it, and Close releases it.
type Reply struct {
	ctx    context.Context
	cancel context.CancelFunc
	stream agent.ModelStream
	prompt backend.Message
}

// Next returns the next piece of text, or io.EOF after the last one. A reply that breaks, times out or is aborted
// returns the error that ended it.
func (r *Reply) Next() (string, error) {
	for {
		chunk, err := r.stream.Next()
		if err != nil {
			return "", callError(r.ctx, err)
		}
		if chunk.Text != "" {
			return chunk.Text, nil
		}
	}
}

// Prompt is the user turn this reply answers, as text, for a session to keep.
func (r *Reply) Prompt() backend.Message {
	return r.prompt
}

// Abort stops the reply. The next call to Next returns the cancellation.
func (r *Reply) Abort() {
	r.cancel()
}

// Close stops the reply if it is still running and releases its connection. It is safe to call more than once.
func (r *Reply) Close() {
	_ = r.stream.Close()
	r.cancel()
}
