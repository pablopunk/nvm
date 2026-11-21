package chat

import "slices"

// NewChat opens a chat of kind that uses tier. It is not recorded until its first message is written, so a chat that is
// opened and left empty leaves nothing in the history.
func (s *Service) NewChat(kind Kind, tier Tier) (Chat, error) {
	now := s.opts.Clock.Now()
	chat := Chat{ID: newChatID(), Kind: kind, Tier: tier, CreatedAt: now, UpdatedAt: now}
	s.cache.putDraft(chat)
	return chat, nil
}

// mutateDraft applies change to a chat nobody has recorded yet. The chat is recorded once it has a message, so an
// opened chat that is left empty never reaches the store.
func (s *Service) mutateDraft(draft Chat, change func(*Chat)) error {
	next := draft
	next.Messages = slices.Clone(draft.Messages)
	change(&next)
	next.UpdatedAt = s.opts.Clock.Now()
	if len(next.Messages) == 0 {
		s.cache.putDraft(next)
		return nil
	}
	if err := s.opts.Store.Insert(next); err != nil {
		return err
	}
	s.cache.commitDraft(next)
	return nil
}
