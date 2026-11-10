package chat

// BuilderTurn identifies one builder toolset. Its nonzero size gives each allocation a distinct identity.
// It contains no model-visible authority and is accepted only by the run holding this exact token.
type BuilderTurn struct{ marker byte }

func NewBuilderTurn() *BuilderTurn { return &BuilderTurn{} }

// Bind records that the chat of kind builder writes or edits the extension id. An extension has one chat, so it refuses
// an extension another chat owns. It refuses a busy chat, since the instruction of its next turn depends on the binding.
func (s *Service) Bind(id, extension string) error {
	if s.Busy(id) {
		return ErrBusy
	}
	if extension != "" {
		if s.extensionClaimedElsewhere(id, extension) {
			return ErrExtensionHasChat
		}
		for _, owner := range s.Owners(extension) {
			if owner.ID != id {
				return ErrExtensionHasChat
			}
		}
	}
	return s.mutate(id, func(c *Chat) { c.Extension = extension })
}

// ReserveExtension reserves a builder target for the active turn. Like binding, this runs on the UI
// thread. Other chats cannot bind or reserve it until the turn settles; Stop retains the reservation
// while the worker drains. The tool context governs cancellation at publication admission.
func (s *Service) ReserveExtension(id, extension string, turn *BuilderTurn) error {
	r, busy := s.runs[id]
	c, known := s.Chat(id)
	if !busy || !known || c.Kind != KindBuilder || extension == "" || s.doomed[id] || turn == nil || r.builderTurn != turn {
		return ErrNotFound
	}
	if c.Extension != "" && c.Extension != extension {
		return ErrExtensionHasChat
	}
	if s.extensionClaimedElsewhere(id, extension) {
		return ErrExtensionHasChat
	}
	for _, owner := range s.Owners(extension) {
		if owner.ID != id {
			return ErrExtensionHasChat
		}
	}
	if r.extensionClaims == nil {
		r.extensionClaims = make(map[string]bool)
	}
	r.extensionClaims[extension] = true
	return nil
}

func (s *Service) extensionClaimedElsewhere(id, extension string) bool {
	for owner, r := range s.runs {
		if owner != id && r.extensionClaims[extension] {
			return true
		}
	}
	return false
}

// CommitExtension records an already published target before its worker returns. Unlike Bind it is
// allowed during the matching reserved turn, including while deletion waits for that worker to drain.
// This keeps the normal chat-removal policy informed even when settlement skips OnSettled.
func (s *Service) CommitExtension(id, extension string, turn *BuilderTurn) error {
	r := s.runs[id]
	c, known := s.Chat(id)
	if r == nil || turn == nil || r.builderTurn != turn || !r.extensionClaims[extension] || !known || c.Kind != KindBuilder {
		return ErrNotFound
	}
	if c.Extension != "" && c.Extension != extension {
		return ErrExtensionHasChat
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
