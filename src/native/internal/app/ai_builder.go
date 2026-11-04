package app

import (
	"log"

	"github.com/pablopunk/nvm/native/internal/ai/builder"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
)

// newBuilder wires the extension builder to the chat service, the extension manager and the UI thread.
func (p *App) newBuilder() *builder.Workshop {
	return builder.New(p.chats, builderExtensions{app: p}, p.onUI)
}

// builderView opens a new builder chat, where the user describes the extension.
func (p *App) builderView() *palette.View { return p.builderViewWith("") }

// builderToolset returns the tools of a builder chat for one turn: none until the builder exists.
func (p *App) builderToolset(chatID string) chat.Toolset {
	if p.builder == nil {
		return chat.Toolset{}
	}
	return p.builder.Toolset(chatID)
}

// builderViewWith opens the extension builder and, given a prompt, sends it at once ("Automate ...").
func (p *App) builderViewWith(prompt string) *palette.View {
	return p.openChat(chat.KindBuilder, chat.TierSmart, prompt)
}

// builderEditView opens a builder chat bound to the extension id, so its replies change that extension. The review
// of the change is the same as for a new one: it is installed enabled.
func (p *App) builderEditView(id string) *palette.View {
	if p.chats == nil {
		return unavailableChatView()
	}
	record, err := p.chats.NewChat(chat.KindBuilder, chat.TierSmart)
	if err != nil {
		log.Println("builder: could not start an edit:", err)
		return unavailableChatView()
	}
	if err := p.chats.Bind(record.ID, id); err != nil {
		log.Println("builder: could not bind the edit to its extension:", err)
	}
	return aichat.ChatView(p.chatDeps(), record.ID)
}

// ownedByBuilder reports whether a builder chat wrote the extension id, so the AI may change it.
func (p *App) ownedByBuilder(id string) bool {
	return p.chats != nil && len(p.chats.Owners(id)) > 0
}

// builderReview is the review page of the extension a builder chat wrote, or nil when it is no longer installed.
func (p *App) builderReview(id string) *palette.View {
	ctl := p.currentExtensionController()
	if ctl == nil {
		return nil
	}
	for _, listing := range ctl.List() {
		if listing.ID == id {
			return p.extensionReviewView(ctl, listing)
		}
	}
	return nil
}
