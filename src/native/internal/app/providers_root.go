package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// rootInput is what the root's providers read. The palette captures it on the UI thread when a query
// starts and the providers work on it on other goroutines, so nothing in it is edited while they run: maps
// are copied, the snippets are cloned, and the slices the UI thread replaces as a whole (apps,
// extensions, file paths) are shared as they are. The clipboard history locks its own state, so it is shared too.
type rootInput struct {
	learn      *usage.Learner
	apps       []appfeat.App
	clip       *clipEngine
	clipInRoot bool
	clipSearch bool
	// updatePrompt is whether the root shows an update prompt, which replaces the Check for Updates row, as in 1.x.
	updatePrompt bool
	chats        []chatHead
	timerOn      bool
	// shortcuts maps each root action that has a keyboard shortcut to its accelerator.
	shortcuts map[string]string
	// aliases maps each root action to the phrases that find it in the root search.
	aliases map[string][]string
}

// chatHead is what the conversation rows read from a chat: its id, its kind, its title and its last use.
type chatHead struct {
	id       string
	kind     chat.Kind
	title    string
	lastUsed int64
}

// chatLastUsed is the later of a chat's creation and its last change, the time 1.x sorts AI builder rows by
// (src/app/extensions/ai-builder.ts:31). A chat with no time has none.
func chatLastUsed(created, updated time.Time) int64 {
	latest := created
	if updated.After(latest) {
		latest = updated
	}
	if latest.IsZero() {
		return 0
	}
	return latest.UnixMilli()
}

// captureRoot copies the state the root's providers need. It runs on the UI thread.
func (p *App) captureRoot() *rootInput {
	chats := p.chatHeads()
	return &rootInput{
		learn:        p.learn,
		apps:         p.appsSnapshot(),
		clip:         p.clip,
		clipInRoot:   !p.S.ClipRootOff,
		clipSearch:   !p.S.ClipRootSearchOff,
		updatePrompt: rootShowsUpdate(p.updateSnapshot().State),
		chats:        chats,
		timerOn:      p.timer != nil,
		shortcuts:    p.boundShortcuts(),
		aliases:      p.aliasStore().All(),
	}
}

// rootProviders builds the root's providers over a fresh capture of the state. The palette calls it on the UI
// thread at the start of every query.
func (p *App) rootProviders() []search.Provider {
	in := p.captureRoot()
	out := make([]search.Provider, 0, len(rootSources))
	for _, source := range rootSources {
		if source.provider != nil {
			out = append(out, source.provider(p, in))
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
		heads = append(heads, chatHead{id: record.ID, kind: record.Kind, title: record.Title, lastUsed: chatLastUsed(record.CreatedAt, record.UpdatedAt)})
	}
	return heads
}
