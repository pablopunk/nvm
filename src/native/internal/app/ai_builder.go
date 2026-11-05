package app

import (
	"github.com/pablopunk/nvm/native/internal/ai/builder"
	"github.com/pablopunk/nvm/native/internal/ai/chat"
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
