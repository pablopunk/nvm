package app

import (
	"context"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/search"
)

// rootInput is what the root's providers read. The palette captures it on the UI thread when a query
// starts and the providers work on it on other goroutines, so nothing in it is edited while they run: maps
// are copied, the snippets and quicklinks are cloned, and the slices the UI thread replaces as a whole (apps,
// extensions, file paths) are shared as they are. The clipboard history locks its own state, so it is shared too.
type rootInput struct {
	engine     engine
	quicklinks []Quicklink
	apps       []appfeat.App
	clip       *clipEngine
	clipInRoot bool
	clipSearch bool
	chats      []chatHead
	timerOn    bool
}

// chatHead is what the conversation rows read from a chat: its id, its title and whether it is a builder chat.
type chatHead struct {
	id      string
	title   string
	builder bool
}

// captureRoot copies the state the root's providers need. It runs on the UI thread.
func (p *App) captureRoot() *rootInput {
	chats := p.chatHeads()
	return &rootInput{
		engine:     p.engine(),
		quicklinks: slices.Clone(p.S.Quicklinks),
		apps:       p.appsSnapshot(),
		clip:       p.clip,
		clipInRoot: !p.S.ClipRootOff,
		clipSearch: !p.S.ClipRootSearchOff,
		chats:      chats,
		timerOn:    p.timer != nil,
	}
}

// rootProviders builds the root's providers over a fresh capture of the state. The palette calls it on the UI
// thread at the start of every query.
func (p *App) rootProviders() []search.Provider {
	in := p.captureRoot()
	out := make([]search.Provider, 0, len(rootSources))
	for _, source := range rootSources {
		if source.provider != nil {
			out = append(out, source.provider(p))
			continue
		}
		rows := source.rows
		out = append(out, search.NewRowsProvider(source.id, func(_ context.Context, q string) []search.Item {
			return rows(p, in, q)
		}))
	}
	return out
}

// chatHeads lists the saved conversations for the root's rows, most recently changed first. It reads the chat
// service, so it runs on the UI thread.
func (p *App) chatHeads() []chatHead {
	if p.chats == nil {
		return nil
	}
	records := p.chats.Chats()
	heads := make([]chatHead, 0, len(records))
	for _, record := range records {
		heads = append(heads, chatHead{id: record.ID, title: record.Title, builder: record.Kind == chat.KindBuilder})
	}
	return heads
}
