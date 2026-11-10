package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// extensionBuilder serves the builder reads and the ownership reads to the built-in extensions. The chats are the
// host's: every read runs on the UI thread, where the chat cache lives.
type extensionBuilder struct {
	p *App
}

// Chats lists the builder chats.
func (b extensionBuilder) Chats() []api.ChatSummary {
	var builders []api.ChatSummary
	for _, summary := range extensionChats(b).List() {
		if summary.Kind == string(chat.KindBuilder) {
			builders = append(builders, summary)
		}
	}
	return builders
}

// Chat returns one builder chat with its transcript.
func (b extensionBuilder) Chat(chatID string) (api.ChatDetail, bool) {
	detail, ok := extensionChats(b).Get(chatID)
	if !ok || detail.Kind != string(chat.KindBuilder) {
		return api.ChatDetail{}, false
	}
	return detail, true
}

// ExtensionPreview is the preview view of an installed extension, which a builder chat shows beside its transcript.
func (b extensionBuilder) ExtensionPreview(extension string) (api.View, bool) {
	m := b.p.extensionManager()
	if m == nil {
		return api.View{}, false
	}
	if _, installed := b.p.installedListing(extension); !installed {
		return api.View{}, false
	}
	view := extensionPreviewView(m, extension)
	view.Presentation = api.PresentSidePreview
	return view, true
}

// Cleanup removes the expired conversations and says how many it removed; builder chats are kept.
func (b extensionBuilder) Cleanup() (int, error) {
	var removed int
	var err error
	b.p.onChatsThread(func() {
		if b.p.chats == nil {
			err = errChatsUnavailable
			return
		}
		ids, expireErr := b.p.chats.Expire()
		removed, err = len(ids), expireErr
	})
	return removed, err
}

// OwnerOf returns the builder chat that owns the extension, when one does.
func (b extensionBuilder) OwnerOf(extension string) (string, bool) {
	var chatID string
	var found bool
	b.p.onChatsThread(func() {
		if b.p.chats == nil {
			return
		}
		if owners := b.p.chats.Owners(extension); len(owners) > 0 {
			chatID, found = owners[0].ID, true
		}
	})
	return chatID, found
}

// FilesForChat lists the extension a builder chat is bound to.
func (b extensionBuilder) FilesForChat(chatID string) []string {
	record, ok := b.record(chatID)
	if !ok || record.Extension == "" {
		return nil
	}
	return []string{record.Extension}
}

// CanWrite reports whether a builder chat is bound to the extension, so it may change it.
func (b extensionBuilder) CanWrite(extension, chatID string) bool {
	record, ok := b.record(chatID)
	return ok && record.Kind == chat.KindBuilder && record.Extension == extension
}

// record reads one chat from the store on the UI thread.
func (b extensionBuilder) record(chatID string) (chat.Chat, bool) {
	var record chat.Chat
	var ok bool
	b.p.onChatsThread(func() {
		if b.p.chats != nil {
			record, ok = b.p.chats.Chat(chatID)
		}
	})
	return record, ok
}
