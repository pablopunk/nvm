package chat

import (
	"errors"
	"reflect"
	"slices"
)

// ErrStaleFork means the source changed since the user chose a message.
var ErrStaleFork = errors.New("chat: conversation changed; choose the message again")

// ForkReply starts a new conversation through the user request answered by index. The original is never changed.
// expected is the complete snapshot the selection came from; callers must retain it until invoking the action.
func (s *Service) ForkReply(expected Chat, index int) (Chat, error) {
	source, ok := s.Chat(expected.ID)
	if !ok {
		return Chat{}, ErrNotFound
	}
	if s.Busy(source.ID) {
		return Chat{}, ErrBusy
	}
	if !reflect.DeepEqual(source, expected) {
		return Chat{}, ErrStaleFork
	}
	if source.Kind != KindConversation || index < 0 || index >= len(source.Messages) {
		return Chat{}, ErrNothingToAsk
	}
	chosen := source.Messages[index]
	if chosen.Author != AuthorAssistant && chosen.Author != AuthorNotice {
		return Chat{}, ErrNothingToAsk
	}
	user := -1
	for i := index - 1; i >= 0; i-- {
		if source.Messages[i].Author == AuthorUser {
			user = i
			break
		}
	}
	if user < 0 {
		return Chat{}, ErrNothingToAsk
	}
	for _, message := range source.Messages[:user+1] {
		if message.Status == StatusStreaming || (message.Author == AuthorTool && toolStillRunning(message.Status)) {
			return Chat{}, ErrBusy
		}
	}
	fork, err := s.NewChat(KindConversation, source.Tier)
	if err != nil {
		return Chat{}, err
	}
	err = s.mutate(fork.ID, func(c *Chat) {
		c.Title = source.Title
		c.Messages = slices.Clone(source.Messages[:user+1])
	})
	if err != nil {
		s.cache.dropDraft(fork.ID)
		return Chat{}, err
	}
	if err = s.startTurn(fork.ID); err != nil {
		return fork, err
	}
	fork, _ = s.Chat(fork.ID)
	return fork, nil
}
