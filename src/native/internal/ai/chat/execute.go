package chat

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
)

// execute runs one turn off the UI thread and hands its outcome to the UI thread to settle.
func (s *Service) execute(id string, r *run, tier Tier, request []backend.Message) {
	outcome := s.converse(id, r, tier, request)
	s.opts.Dispatch(func() { s.settle(id, r, outcome) })
}

// converse resolves the tier's route, checks that this version can speak it, and streams the reply. Each text piece
// reaches the transcript through Dispatch, in order.
func (s *Service) converse(id string, r *run, tier Tier, request []backend.Message) Outcome {
	client := s.opts.Client()
	if client == nil {
		return failed(ErrNotSignedIn)
	}
	descriptor, err := client.ActiveModel(r.ctx, backend.ActiveModelQuery{Role: string(tier), Chars: RequestSize(request)})
	if err != nil {
		return endingOnError(r.ctx, err)
	}
	s.opts.Dispatch(func() { s.models[tier] = descriptor })
	if blocked := blockedRouteError(descriptor); blocked != nil {
		return failed(blocked)
	}
	if !descriptor.SpeaksChatCompletions() {
		return failed(ErrRouteUnsupported)
	}
	stream, err := client.ChatStream(r.ctx, backend.ChatRequest{Role: string(tier), Messages: request})
	if err != nil {
		return endingOnError(r.ctx, err)
	}
	if !r.attach(stream) {
		_ = stream.Close()
		return Outcome{Ending: EndStopped}
	}
	defer func() { _ = stream.Close() }()
	return readReply(r.ctx, stream, func(text string) {
		s.opts.Dispatch(func() { s.appendText(id, text) })
	})
}

// paymentRequired is the HTTP status the backend refuses a paused route with.
const paymentRequired = 402

// blockedRouteError is the refusal for a route the backend marks as paused by the free spend cap, or nil. It is
// shaped as the refusal the backend would have sent, so the same notice and account note apply.
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

// failed is the outcome of a turn that ended with err.
func failed(err error) Outcome {
	return Outcome{Ending: EndFailed, Err: err}
}

// ErrEmptyReply is the failure of a reply that finished without any text.
var ErrEmptyReply = errors.New("chat: the reply had no text")

// appendText adds a piece of reply text to the streaming message of the chat.
func (s *Service) appendText(id, text string) {
	_ = s.opts.Store.Mutate(id, func(c *Chat) {
		if index := streamingIndex(c); index >= 0 {
			c.Messages[index].Text += text
		}
	})
}
