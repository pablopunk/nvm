package app

import (
	"context"
	"maps"
	"slices"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// rootInput is what the root's providers read. The palette captures it on the UI thread when a query
// starts and the providers work on it on other goroutines, so nothing in it is edited while they run: maps
// are copied, the snippets and quicklinks are cloned, and the slices the UI thread replaces as a whole (apps,
// extensions, file paths) are shared as they are. The clipboard history locks its own state, so it is shared too.
type rootInput struct {
	engine     engine
	quicklinks []Quicklink
	learn      *usage.Learner
	disabled   map[string]bool
	snippets   []Snippet
	exts       []loadedExt
	apps       []mygo.InstalledApp
	clip       *clipEngine
	chats      []chatHead
	timerOn    bool
	files      []string
	filesDone  bool
}

// chatHead is what the conversation rows read from a chat. The chat itself is opened on the UI thread only.
type chatHead struct {
	chat    *Chat
	title   string
	builder bool
}

// captureRoot copies the state the root's providers need. It runs on the UI thread.
func (p *App) captureRoot() *rootInput {
	chats := make([]chatHead, 0, len(p.chats.Chats))
	for _, ch := range p.chats.Chats {
		chats = append(chats, chatHead{chat: ch, title: ch.Title, builder: ch.Builder})
	}
	paths, done := p.files.snapshot()
	return &rootInput{
		engine:     p.engine(),
		quicklinks: slices.Clone(p.S.Quicklinks),
		learn:      p.learn,
		disabled:   maps.Clone(p.S.Disabled),
		snippets:   slices.Clone(p.snips.Items),
		exts:       p.exts,
		apps:       p.apps,
		clip:       p.clip,
		chats:      chats,
		timerOn:    p.timer != nil,
		files:      paths,
		filesDone:  done,
	}
}

// rootProviders builds the root's providers over a fresh capture of the state. The palette calls it on the UI
// thread at the start of every query.
func (p *App) rootProviders() []search.Provider {
	in := p.captureRoot()
	out := make([]search.Provider, 0, len(rootSources))
	for _, source := range rootSources {
		rows := source.rows
		out = append(out, search.NewRowsProvider(source.id, func(_ context.Context, q string) []search.Item {
			return rows(p, in, q)
		}))
	}
	return out
}
