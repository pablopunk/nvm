package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// aiStream is one reply the extension pulls. A goroutine reads the reply into a queue of events; stream-next takes
// what is queued and waits when nothing is.
type aiStream struct {
	id      string
	reply   *extai.Reply
	session *aiSession
	prompt  backend.Message
	wake    chan struct{}
	mu      sync.Mutex
	queue   []api.AIStreamEvent
	text    strings.Builder
	ended   bool
	failure *api.Failure
}

func newAIStream(id string, reply *extai.Reply, session *aiSession, prompt backend.Message) *aiStream {
	return &aiStream{id: id, reply: reply, session: session, prompt: prompt, wake: make(chan struct{}, 1)}
}

// run reads the reply to its end. It closes the reply when it returns.
func (s *aiStream) run() {
	defer s.reply.Close()
	s.enqueue(api.AIStreamEvent{Type: api.AIEventStart})
	for {
		piece, err := s.reply.Next()
		if errors.Is(err, io.EOF) {
			s.finish()
			return
		}
		if err != nil {
			s.fail(err)
			return
		}
		s.delta(piece)
	}
}

func (s *aiStream) delta(piece string) {
	s.mu.Lock()
	s.text.WriteString(piece)
	s.queue = append(s.queue, api.AIStreamEvent{Type: api.AIEventDelta, Text: piece})
	s.mu.Unlock()
	s.signal()
}

// finish ends the stream with its full reply and keeps the turn in the session it belongs to.
func (s *aiStream) finish() {
	s.mu.Lock()
	s.ended = true
	reply := s.text.String()
	s.queue = append(s.queue, api.AIStreamEvent{Type: api.AIEventDone})
	s.mu.Unlock()
	if s.session != nil {
		s.session.record(s.prompt, reply)
	}
	s.signal()
}

// fail ends the stream with the failure that stopped it. An abort is reported as aborted, not as an error.
func (s *aiStream) fail(err error) {
	aborted := errors.Is(err, context.Canceled) || backend.IsKind(err, backend.KindCanceled)
	failure := failureOf(aiFailure(err))
	event := api.AIStreamEvent{Type: api.AIEventError, Message: failure.Message}
	if aborted {
		failure = api.Failure{Code: api.CodeCancelled, Message: "Cancelled."}
		event = api.AIStreamEvent{Type: api.AIEventAborted}
	}
	s.mu.Lock()
	s.ended = true
	s.failure = &failure
	s.queue = append(s.queue, event)
	s.mu.Unlock()
	s.signal()
}

func (s *aiStream) enqueue(event api.AIStreamEvent) {
	s.mu.Lock()
	s.queue = append(s.queue, event)
	s.mu.Unlock()
	s.signal()
}

// take returns the events queued since the last take. ready is false when nothing has happened yet.
func (s *aiStream) take() (batch protocol.AIStreamBatch, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 && !s.ended {
		return protocol.AIStreamBatch{}, false
	}
	batch = protocol.AIStreamBatch{Events: s.queue, Ended: s.ended}
	s.queue = nil
	if s.ended {
		batch.Text = s.text.String()
		batch.Failure = s.failure
	}
	return batch, true
}

// signal wakes a pull that waits for events. It never blocks: one pending wake is enough.
func (s *aiStream) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// openAIStream starts a streamed reply and returns its id. The reply is read in the background; stream-next pulls it.
func (h *Handler) openAIStream(raw json.RawMessage) (protocol.AIStreamOpened, error) {
	var args protocol.AIAskArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIStreamOpened{}, err
	}
	if err := h.requireAIReads(args.Options.Attachments); err != nil {
		return protocol.AIStreamOpened{}, err
	}
	if err := h.allowAICall(); err != nil {
		return protocol.AIStreamOpened{}, err
	}
	call, session, err := h.streamCall(args)
	if err != nil {
		return protocol.AIStreamOpened{}, err
	}
	reply, err := h.deps.AI.Open(h.ai.ctx, call, extai.StreamTimeout)
	if err != nil {
		return protocol.AIStreamOpened{}, aiFailure(err)
	}
	stream, err := h.ai.addStream(reply, session, reply.Prompt())
	if err != nil {
		return protocol.AIStreamOpened{}, err
	}
	safego.Go("ai-stream", func(context.Context) { stream.run() })
	return protocol.AIStreamOpened{ID: stream.id}, nil
}

// streamCall builds the call of a stream, with the history of its session when it has one.
func (h *Handler) streamCall(args protocol.AIAskArgs) (extai.Call, *aiSession, error) {
	if args.Session == "" {
		return extai.Call{Options: args.Options, Prompt: args.Prompt}, nil, nil
	}
	session, err := h.ai.session(args.Session, args.Options)
	if err != nil {
		return extai.Call{}, nil, err
	}
	return extai.Call{Options: session.optionsFor(args.Options), Prompt: args.Prompt, History: session.history()}, session, nil
}

// nextAIStream returns the events of a stream since its last pull, waiting for the next one when there are none. The
// batch that ends the stream also ends its id.
func (h *Handler) nextAIStream(ctx context.Context, raw json.RawMessage) (protocol.AIStreamBatch, error) {
	var args protocol.AIStreamRef
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIStreamBatch{}, err
	}
	stream, ok := h.ai.stream(args.ID)
	if !ok {
		return protocol.AIStreamBatch{}, invalid("there is no AI stream %q", args.ID)
	}
	for {
		batch, ready := stream.take()
		if ready {
			if batch.Ended {
				h.ai.removeStream(args.ID)
			}
			return batch, nil
		}
		select {
		case <-stream.wake:
		case <-ctx.Done():
			return protocol.AIStreamBatch{}, ctx.Err()
		}
	}
}

// abortAIStream stops a stream. The stream reports the abort as its last event.
func (h *Handler) abortAIStream(raw json.RawMessage) (struct{}, error) {
	var args protocol.AIStreamRef
	if err := decodeArgs(raw, &args); err != nil {
		return struct{}{}, err
	}
	if stream, ok := h.ai.stream(args.ID); ok {
		stream.reply.Abort()
	}
	return struct{}{}, nil
}

// resetAISession forgets the turns of a session. The next call under its id starts a new session.
func (h *Handler) resetAISession(raw json.RawMessage) (struct{}, error) {
	var args protocol.KeyArgs
	if err := decodeArgs(raw, &args); err != nil {
		return struct{}{}, err
	}
	h.ai.resetSession(args.Key)
	return struct{}{}, nil
}
