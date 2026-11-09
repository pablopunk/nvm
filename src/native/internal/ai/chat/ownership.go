package chat

// Bind records that the chat of kind builder writes or edits the extension id. An extension has one chat, so it refuses
// an extension another chat owns. It refuses a busy chat, since the instruction of its next turn depends on the binding.
func (s *Service) Bind(id, extension string) error {
	if s.Busy(id) {
		return ErrBusy
	}
	if extension != "" {
		for _, owner := range s.Owners(extension) {
			if owner.ID != id {
				return ErrExtensionHasChat
			}
		}
	}
	return s.mutate(id, func(c *Chat) { c.Extension = extension })
}

// Owners returns the builder chats bound to the extension id: those with a transcript, most recently changed first, then
// any opened but not used yet.
func (s *Service) Owners(extension string) []Chat {
	var owners []Chat
	for _, chat := range s.Chats() {
		if chat.Kind == KindBuilder && chat.Extension == extension {
			owners = append(owners, chat)
		}
	}
	for _, chat := range s.cache.drafts {
		if chat.Kind == KindBuilder && chat.Extension == extension {
			owners = append(owners, chat)
		}
	}
	return owners
}
