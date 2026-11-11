package chat

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// EventStream is the reply stream a one-shot completion reads. *backend.Stream satisfies it.
type EventStream interface {
	Next() (backend.Event, error)
	Close() error
}

// CompletionSource is the backend a one-shot completion asks: the route of the fast tier, and a streamed reply on it.
// The app adapts its backend client to it, so tests use a fake.
type CompletionSource interface {
	ActiveModel(ctx context.Context, query backend.ActiveModelQuery) (backend.ModelDescriptor, error)
	ChatStream(ctx context.Context, req backend.ChatRequest) (EventStream, error)
}

// errIncompleteReply marks a reply whose stream ended before its end marker.
var errIncompleteReply = errors.New("chat: the fast model's reply ended early")

// Complete asks the fast tier for one reply to system and user and returns its text. It keeps no chat record, so a
// one-shot task such as dictation cleanup never shows up in AI Chats. The route is checked first: a route the spend
// cap paused or this version cannot read is refused before any text is sent.
func Complete(ctx context.Context, source CompletionSource, system, user string) (string, error) {
	return completeBounded(ctx, source, system, user, 0)
}

// CompleteBounded refuses a streamed answer above maxBytes, before retaining it in memory.
func CompleteBounded(ctx context.Context, source CompletionSource, system, user string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		return "", errors.New("chat: completion limit must be positive")
	}
	return completeBounded(ctx, source, system, user, maxBytes)
}
func completeBounded(ctx context.Context, source CompletionSource, system, user string, maxBytes int) (string, error) {
	descriptor, err := source.ActiveModel(ctx, backend.ActiveModelQuery{Role: backend.RoleFast, Chars: len(system) + len(user)})
	if err != nil {
		return "", err
	}
	if blocked := blockedRouteError(descriptor); blocked != nil {
		return "", blocked
	}
	if !descriptor.SpeaksChatCompletions() {
		return "", ErrRouteUnsupported
	}
	stream, err := source.ChatStream(ctx, backend.ChatRequest{
		Role: backend.RoleFast,
		Messages: []backend.Message{
			{Role: backend.RoleSystem, Content: backend.TextContent(system)},
			{Role: backend.RoleUser, Content: backend.TextContent(user)},
		},
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = stream.Close() }()
	return readCompletion(stream, maxBytes)
}

// readCompletion joins the content pieces of a reply. Only the end marker completes a reply: a stream that closes
// without one is a broken reply, and an error event ends it with the backend's error.
func readCompletion(stream EventStream, maxBytes int) (string, error) {
	var text strings.Builder
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			return "", errIncompleteReply
		}
		if err != nil {
			return "", err
		}
		switch event.Type {
		case backend.EventContent:
			if maxBytes > 0 && len(event.Text) > maxBytes-text.Len() {
				return "", errors.New("chat: completion exceeds limit")
			}
			text.WriteString(event.Text)
		case backend.EventDone:
			return text.String(), nil
		case backend.EventError:
			if event.Err != nil {
				return "", event.Err
			}
			return "", errIncompleteReply
		}
	}
}
