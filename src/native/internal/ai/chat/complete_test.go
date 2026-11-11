package chat

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// fakeSource answers the route query with a descriptor and streams the scripted events.
type fakeSource struct {
	descriptor backend.ModelDescriptor
	routeErr   error
	events     []backend.Event
	streamErr  error
	request    backend.ChatRequest
	query      backend.ActiveModelQuery
}

func (f *fakeSource) ActiveModel(_ context.Context, query backend.ActiveModelQuery) (backend.ModelDescriptor, error) {
	f.query = query
	return f.descriptor, f.routeErr
}

func (f *fakeSource) ChatStream(_ context.Context, req backend.ChatRequest) (EventStream, error) {
	f.request = req
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return &scriptedEvents{events: f.events}, nil
}

// scriptedEvents yields its events in order, then io.EOF, as a backend stream does after its end marker.
type scriptedEvents struct {
	events []backend.Event
}

func (s *scriptedEvents) Next() (backend.Event, error) {
	if len(s.events) == 0 {
		return backend.Event{}, io.EOF
	}
	event := s.events[0]
	s.events = s.events[1:]
	return event, nil
}

func (s *scriptedEvents) Close() error { return nil }

func openAIRoute() backend.ModelDescriptor {
	return backend.ModelDescriptor{API: "openai-completions", Notice: "ok"}
}

func TestCompleteJoinsTheContentPiecesOfAFinishedReply(t *testing.T) {
	source := &fakeSource{descriptor: openAIRoute(), events: []backend.Event{
		{Type: backend.EventContent, Text: "Hello, "},
		{Type: backend.EventContent, Text: "world."},
		{Type: backend.EventDone},
	}}
	got, err := Complete(context.Background(), source, "sys", "usr")
	if err != nil || got != "Hello, world." {
		t.Fatalf("Complete = %q, %v; want the joined reply", got, err)
	}
	if source.query.Role != backend.RoleFast {
		t.Fatalf("route role = %q, want the fast tier", source.query.Role)
	}
	if source.request.Role != backend.RoleFast || len(source.request.Messages) != 2 ||
		source.request.Messages[0].Role != backend.RoleSystem || source.request.Messages[1].Role != backend.RoleUser {
		t.Fatalf("request = %+v, want a fast-tier system then user message", source.request)
	}
}

func TestCompleteRefusesARouteThatCannotBeUsedBeforeSendingAnything(t *testing.T) {
	paused := openAIRoute()
	paused.Notice = "blocked"
	paused.NoticeReason = "free_spend_cap_reached"
	tests := map[string]struct {
		descriptor backend.ModelDescriptor
		routeErr   error
		want       error
	}{
		"route lookup fails": {openAIRoute(), errors.New("offline"), nil},
		"route is paused":    {paused, nil, nil},
		"wire format":        {backend.ModelDescriptor{API: "anthropic-messages"}, nil, ErrRouteUnsupported},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			source := &fakeSource{descriptor: test.descriptor, routeErr: test.routeErr, events: []backend.Event{{Type: backend.EventDone}}}
			_, err := Complete(context.Background(), source, "sys", "usr")
			if err == nil {
				t.Fatalf("Complete error = nil, want a refusal")
			}
			if test.want != nil && !errors.Is(err, test.want) {
				t.Fatalf("Complete error = %v, want %v", err, test.want)
			}
			if source.request.Role != "" {
				t.Fatalf("a refused route still sent a request: %+v", source.request)
			}
		})
	}
}

func TestCompleteReportsAStreamErrorAndAReplyWithoutItsEndMarker(t *testing.T) {
	failure := &backend.Error{Kind: backend.KindServer, Message: "upstream failed"}
	errorSource := &fakeSource{descriptor: openAIRoute(), events: []backend.Event{{Type: backend.EventContent, Text: "x"}, {Type: backend.EventError, Err: failure}}}
	if _, err := Complete(context.Background(), errorSource, "sys", "usr"); !errors.Is(err, failure) {
		t.Fatalf("error event = %v, want the backend error", err)
	}
	broken := &fakeSource{descriptor: openAIRoute(), events: []backend.Event{{Type: backend.EventContent, Text: "cut"}}}
	if _, err := Complete(context.Background(), broken, "sys", "usr"); !errors.Is(err, errIncompleteReply) {
		t.Fatalf("a reply without its end marker = %v, want errIncompleteReply", err)
	}
}

func TestCompleteBoundedRejectsOverlongStreamingAnswer(t *testing.T) {
	stream := &scriptedEvents{events: []backend.Event{{Type: backend.EventContent, Text: "abc"}, {Type: backend.EventContent, Text: "def"}, {Type: backend.EventDone}}}
	if _, err := readCompletion(stream, 5); err == nil {
		t.Fatal("accepted overlong completion")
	}
}
