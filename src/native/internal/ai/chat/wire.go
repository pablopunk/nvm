package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
)

// paymentRequired is the HTTP status the backend refuses a paused route with.
const paymentRequired = 402

// routeModel is the model of one turn: the backend route of a tier, resolved once when the turn first streams, and a
// chat request per model turn of the loop. It never names a model; the backend routes the tier.
type routeModel struct {
	client    func() *backend.Client
	tier      Tier
	dispatch  func(fn func())
	note      func(tier Tier, descriptor backend.ModelDescriptor)
	route     *backend.ModelDescriptor
	preflight func(ctx context.Context) error
}

// Stream opens one model turn over messages, offering tools.
func (m *routeModel) Stream(ctx context.Context, messages []backend.Message, tools json.RawMessage) (agent.ModelStream, error) {
	client := m.client()
	if client == nil {
		return nil, ErrNotSignedIn
	}
	if m.preflight != nil {
		if err := m.preflight(ctx); err != nil {
			return nil, err
		}
	}
	if m.route == nil {
		if err := m.resolve(ctx, client, messages); err != nil {
			return nil, err
		}
	}
	stream, err := client.ChatStream(ctx, backend.ChatRequest{Role: string(m.tier), Messages: messages, Tools: tools})
	if err != nil {
		return nil, err
	}
	return newWireStream(ctx, stream), nil
}

// resolve reads the route of the tier and refuses one this version cannot speak.
func (m *routeModel) resolve(ctx context.Context, client *backend.Client, messages []backend.Message) error {
	descriptor, err := client.ActiveModel(ctx, backend.ActiveModelQuery{Role: string(m.tier), Chars: RequestSize(messages)})
	if err != nil {
		return err
	}
	tier, shown := m.tier, descriptor
	m.dispatch(func() { m.note(tier, shown) })
	if blocked := blockedRouteError(descriptor); blocked != nil {
		return blocked
	}
	if !descriptor.SpeaksChatCompletions() {
		return ErrRouteUnsupported
	}
	m.route = &descriptor
	return nil
}

// blockedRouteError is the refusal for a route the backend marks as paused by the free spend cap, or nil. It is shaped
// as the refusal the backend would have sent, so the same notice and account note apply.
func blockedRouteError(descriptor backend.ModelDescriptor) error {
	if _, paused := auth.SpendCapNoticeFromModel(descriptor); !paused {
		return nil
	}
	return &backend.Error{
		Kind:    backend.KindRejected,
		Status:  paymentRequired,
		Code:    "free_spend_cap_reached",
		Message: "the route is paused by the free spend cap",
	}
}

// wireStream turns the backend's events into model chunks. A complete reply ends with io.EOF; a stream that closes
// before its end marker is a broken reply, not a finished one. Cancelling the turn closes the stream, which unblocks a
// read that is waiting for the network.
type wireStream struct {
	stream *backend.Stream
	stop   func() bool
}

// newWireStream wraps stream so that cancelling ctx closes it.
func newWireStream(ctx context.Context, stream *backend.Stream) *wireStream {
	return &wireStream{stream: stream, stop: context.AfterFunc(ctx, func() { _ = stream.Close() })}
}

// Next returns the next chunk of the reply, skipping events a turn does not use.
func (w *wireStream) Next() (agent.ModelChunk, error) {
	for {
		event, err := w.stream.Next()
		if errors.Is(err, io.EOF) {
			return agent.ModelChunk{}, &backend.Error{Kind: backend.KindBadResponse, Message: "stream ended before the end-of-stream marker"}
		}
		if err != nil {
			return agent.ModelChunk{}, err
		}
		if chunk, ok, err := chunkOf(event); ok || err != nil {
			return chunk, err
		}
	}
}

// Close releases the backend stream.
func (w *wireStream) Close() error {
	w.stop()
	return w.stream.Close()
}

// chunkOf maps one backend event to a model chunk. ok is false for an event a turn ignores; a done event ends the
// reply with io.EOF.
func chunkOf(event backend.Event) (chunk agent.ModelChunk, ok bool, err error) {
	switch event.Type {
	case backend.EventContent:
		return agent.ModelChunk{Text: event.Text}, event.Text != "", nil
	case backend.EventToolCallDelta:
		delta := event.ToolCall
		return agent.ModelChunk{ToolDelta: &delta}, true, nil
	case backend.EventUsage:
		usage := agent.Usage{InputTokens: event.Usage.InputTokens, OutputTokens: event.Usage.OutputTokens}
		return agent.ModelChunk{Usage: &usage}, true, nil
	case backend.EventDone:
		return agent.ModelChunk{}, true, io.EOF
	case backend.EventError:
		if event.Err == nil {
			return agent.ModelChunk{}, true, &backend.Error{Kind: backend.KindBadResponse, Message: "the backend ended the stream with an unknown error"}
		}
		return agent.ModelChunk{}, true, event.Err
	}
	return agent.ModelChunk{}, false, nil
}
