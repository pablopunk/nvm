package extai

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Deps are the ports the calls use. Client returns the backend client of the account, or nil when it is signed out.
// Connected reports the same state without a client. Home stands for a leading "~" in attachment paths.
type Deps struct {
	Client    func() *backend.Client
	Connected func() bool
	Clock     platform.Clock
	Clipboard platform.Clipboard
	Files     platform.FS
	OCR       *ocr.Service
	Home      string
}

// Service makes the model calls. It is safe for concurrent use.
type Service struct {
	deps Deps
}

// New returns the call service over deps.
func New(deps Deps) *Service {
	return &Service{deps: deps}
}

// Available reports whether the account is signed in and can make calls.
func (s *Service) Available() bool {
	return s.deps.Connected != nil && s.deps.Connected()
}

func (s *Service) client() *backend.Client {
	if s.deps.Client == nil {
		return nil
	}
	return s.deps.Client()
}

// Warm resolves the route of the tier in options, so a later call skips no lookup it would not have made.
func (s *Service) Warm(ctx context.Context, options api.AIOptions) error {
	tier, err := TierOf(options.Model)
	if err != nil {
		return err
	}
	if !s.Available() {
		return chat.ErrNotSignedIn
	}
	ctx, cancel := withDeadline(ctx, s.deps.Clock, CallTimeout)
	defer cancel()
	return callError(ctx, chat.NewRoute(s.client, tier).Warm(ctx))
}

// Call is one model call. History holds the earlier turns of a session and is empty for a one-shot call.
type Call struct {
	Options api.AIOptions
	Prompt  string
	History []backend.Message
}

// Completed is a reply that finished: its text, and the user turn that asked it, which a session keeps.
type Completed struct {
	Text   string
	Prompt backend.Message
}

// Complete sends call and waits for the whole reply.
func (s *Service) Complete(ctx context.Context, call Call) (Completed, error) {
	reply, err := s.Open(ctx, call, CallTimeout)
	if err != nil {
		return Completed{}, err
	}
	defer reply.Close()
	var text strings.Builder
	for {
		piece, err := reply.Next()
		if errors.Is(err, io.EOF) {
			return Completed{Text: text.String(), Prompt: reply.Prompt()}, nil
		}
		if err != nil {
			return Completed{}, err
		}
		text.WriteString(piece)
	}
}

// Open sends call and returns the reply as it streams. The reply ends after timeout, and Close releases it.
func (s *Service) Open(ctx context.Context, call Call, timeout time.Duration) (*Reply, error) {
	if !s.Available() {
		return nil, chat.ErrNotSignedIn
	}
	tier, err := TierOf(call.Options.Model)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withDeadline(ctx, s.deps.Clock, timeout)
	turn, err := s.compose(ctx, call)
	if err != nil {
		cancel()
		return nil, callError(ctx, err)
	}
	stream, err := chat.NewRoute(s.client, tier).Stream(ctx, turn.messages)
	if err != nil {
		cancel()
		return nil, callError(ctx, err)
	}
	return &Reply{ctx: ctx, cancel: cancel, stream: stream, prompt: turn.prompt}, nil
}

// composed is the request one call sends: its messages, and the user turn a session keeps.
type composed struct {
	messages []backend.Message
	prompt   backend.Message
}

// compose reads the attachments of call and builds the messages it sends.
func (s *Service) compose(ctx context.Context, call Call) (composed, error) {
	attachments := call.Options.Attachments
	if len(attachments) > MaxAttachments {
		attachments = attachments[:MaxAttachments]
	}
	content, err := s.readAttachments(ctx, attachments, false)
	if err != nil {
		return composed{}, err
	}
	prompt := promptWithContext(call.Prompt, content.text())
	var messages []backend.Message
	if call.Options.System != "" {
		messages = append(messages, backend.Message{Role: backend.RoleSystem, Content: backend.TextContent(call.Options.System)})
	}
	messages = append(messages, call.History...)
	messages = append(messages, userMessage(prompt, content.images))
	return composed{messages: messages, prompt: backend.Message{Role: backend.RoleUser, Content: backend.TextContent(prompt)}}, nil
}
