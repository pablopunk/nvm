package chat

import (
	"context"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Options are what the service runs on. Client returns the backend client of the active account, or nil when
// there is none, and Connected reports whether that account is signed in. Dispatch runs a change on the UI thread;
// the service owns its state on that thread, and only a turn's goroutine works off it, through Dispatch.
type Options struct {
	Store     *Store
	Clock     platform.Clock
	Client    func() *backend.Client
	Connected func() bool
	// Dispatch runs fn on the UI thread, in the order it was called.
	Dispatch func(fn func())
	// NoteError sees every error that ends a turn, so the account can show a spend-cap notice.
	NoteError func(err error)
	// SystemPrompt returns the instruction a chat of kind starts with. Nil means the conversation prompt.
	SystemPrompt func(kind Kind) string
	// OnSettled runs on the UI thread after a turn settles and its transcript is saved.
	OnSettled func(chatID string, outcome Outcome)
}

// Service runs the chats: it sends a message, streams the reply into the transcript, stops or repeats a turn,
// and keeps the last model each tier resolved to. Its state is owned by the UI thread.
type Service struct {
	opts   Options
	cache  cache
	runs   map[string]*run
	models map[Tier]backend.ModelDescriptor
}

// New returns a service over opts. Expired conversations are removed by Expire, which the caller runs at start.
func New(opts Options) *Service {
	if opts.SystemPrompt == nil {
		opts.SystemPrompt = func(Kind) string { return ConversationSystemPrompt }
	}
	s := &Service{opts: opts, runs: map[string]*run{}, models: map[Tier]backend.ModelDescriptor{}}
	s.Reload()
	return s
}

// Reload re-reads every chat from the store into the cache. Use it after the store changed outside the service.
func (s *Service) Reload() {
	all, _ := s.opts.Store.All()
	s.cache.load(all)
}

// AccountChanged re-reads the model routes after the account connected, changed environment or signed out. A
// disconnected account keeps no routes. It runs on the UI thread.
func (s *Service) AccountChanged() {
	if !s.opts.Connected() {
		s.models = map[Tier]backend.ModelDescriptor{}
		return
	}
	s.RefreshModels()
}

// Version counts the changes the service has made to its chats. A view can compare it to skip a rebuild.
func (s *Service) Version() uint64 {
	return s.cache.version
}

// Chats returns every chat, most recently changed first. The chats are copies of the cache's values; their
// messages are shared and must not be modified.
func (s *Service) Chats() []Chat {
	chats := slices.Clone(s.cache.chats)
	slices.SortStableFunc(chats, func(a, b Chat) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return chats
}

// Chat returns the chat with id, without copying its transcript. The Messages slice is shared with the cache and
// must not be modified.
func (s *Service) Chat(id string) (Chat, bool) {
	return s.cache.get(id)
}

// NewChat adds an empty chat of kind that uses tier.
func (s *Service) NewChat(kind Kind, tier Tier) (Chat, error) {
	chat, err := s.opts.Store.Create(kind, tier)
	if err != nil {
		return Chat{}, err
	}
	s.cache.put(chat)
	return chat, nil
}

// mutate applies change to the chat with id, in the store and in the cache. The change runs once on each copy,
// so it must be a pure function of the chat it is given.
func (s *Service) mutate(id string, change func(*Chat)) error {
	index := s.cache.indexOf(id)
	if index < 0 {
		return ErrNotFound
	}
	now := s.opts.Clock.Now()
	if err := s.opts.Store.Mutate(id, now, change); err != nil {
		return err
	}
	next := s.cache.chats[index]
	next.Messages = slices.Clone(next.Messages)
	change(&next)
	next.UpdatedAt = now
	s.cache.replace(index, next)
	return nil
}

// Busy reports whether a turn of the chat is in flight.
func (s *Service) Busy(id string) bool {
	_, ok := s.runs[id]
	return ok
}

// Delete removes a chat that is not answering.
func (s *Service) Delete(id string) error {
	if s.Busy(id) {
		return ErrBusy
	}
	if err := s.opts.Store.Remove(id); err != nil {
		return err
	}
	s.cache.drop(id)
	return nil
}

// SetTier changes the tier the chat's next turn uses.
func (s *Service) SetTier(id string, tier Tier) error {
	if s.Busy(id) {
		return ErrBusy
	}
	return s.mutate(id, func(c *Chat) { c.Tier = tier })
}

// Model returns the descriptor the backend last resolved tier to, and whether there is one.
func (s *Service) Model(tier Tier) (backend.ModelDescriptor, bool) {
	descriptor, ok := s.models[tier]
	return descriptor, ok
}

// Expire removes the conversations idle past the retention window, except the ones answering now.
func (s *Service) Expire() ([]string, error) {
	removed, err := s.opts.Store.Expire(s.opts.Clock.Now(), s.Busy)
	s.Reload()
	return removed, err
}

// Send adds text as the user's message and starts a reply. Without a connected account the message is kept and a
// sign-in notice follows it, so the transcript shows what happened.
func (s *Service) Send(id, text string) error {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return ErrEmpty
	case s.Busy(id):
		return ErrBusy
	}
	err := s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, s.message(AuthorUser, text, StatusComplete))
		if c.Title == "" {
			c.Title = TitleFor(text)
		}
	})
	if err != nil {
		return err
	}
	return s.startTurn(id)
}

// Note adds an informational notice to the transcript, such as the outcome of an action the chat started.
func (s *Service) Note(id string, notice Notice) error {
	return s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, s.noticeMessage(notice))
	})
}

// Regenerate drops the replies and notices after the last user message and asks again.
func (s *Service) Regenerate(id string) error {
	if s.Busy(id) {
		return ErrBusy
	}
	asked := false
	err := s.mutate(id, func(c *Chat) { asked = truncateAfterLastUser(c) })
	if err != nil {
		return err
	}
	if !asked {
		return ErrNothingToAsk
	}
	return s.startTurn(id)
}

// Stop cancels the turn of the chat. The reply keeps the text it has; the turn settles on the UI thread.
func (s *Service) Stop(id string) {
	if r, ok := s.runs[id]; ok {
		r.stop()
	}
}

// RefreshModels asks the backend which route each tier resolves to, so the palette can name the models. It runs
// off the UI thread and changes nothing when the account is not connected.
func (s *Service) RefreshModels() {
	client := s.opts.Client()
	if client == nil || !s.opts.Connected() {
		return
	}
	go s.resolveTiers(client)
}

// resolveTiers reads the route of each tier and hands each result to the UI thread.
func (s *Service) resolveTiers(client *backend.Client) {
	for _, tier := range []Tier{TierFast, TierSmart} {
		descriptor, err := client.ActiveModel(context.Background(), backend.ActiveModelQuery{Role: string(tier)})
		if err != nil {
			continue
		}
		s.opts.Dispatch(func() { s.models[tier] = descriptor })
	}
}

// startTurn starts a reply to the chat's last message. Without a connected account it writes the sign-in notice.
func (s *Service) startTurn(id string) error {
	chat, ok := s.Chat(id)
	if !ok {
		return ErrNotFound
	}
	if !s.opts.Connected() {
		return s.mutate(id, func(c *Chat) {
			c.Messages = append(c.Messages, s.noticeMessage(SignInNotice()))
		})
	}
	request := Request(s.opts.SystemPrompt(chat.Kind), chat)
	err := s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, s.message(AuthorAssistant, "", StatusStreaming))
	})
	if err != nil {
		return err
	}
	r := newRun()
	s.runs[id] = r
	go s.execute(id, r, chat.Tier, request)
	return nil
}

// message builds a transcript entry stamped with the service's clock.
func (s *Service) message(author Author, text string, status Status) Message {
	return Message{Author: author, Text: text, Status: status, At: s.opts.Clock.Now()}
}

// noticeMessage builds a notice entry for the transcript.
func (s *Service) noticeMessage(notice Notice) Message {
	return Message{Author: AuthorNotice, Text: notice.Message, Status: StatusComplete, Notice: &notice, At: s.opts.Clock.Now()}
}

// truncateAfterLastUser keeps the transcript up to and including its last user message. It reports whether there
// was one to keep.
func truncateAfterLastUser(c *Chat) bool {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Author == AuthorUser {
			c.Messages = c.Messages[:i+1]
			return true
		}
	}
	return false
}
