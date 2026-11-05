package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/builder"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// newBuilder wires the extension builder to the chat service, the extension manager and the UI thread.
func (p *App) newBuilder() *builder.Workshop {
	return builder.New(p.chats, builderExtensions{app: p}, p.onUI)
}

// builderToolset returns the tools of a builder chat for one turn: none until the builder exists.
func (p *App) builderToolset(chatID string) chat.Toolset {
	if p.builder == nil {
		return chat.Toolset{}
	}
	return p.builder.Toolset(chatID)
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
