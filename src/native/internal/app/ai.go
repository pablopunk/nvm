package app

import (
	"context"
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/store"
)

// attachChats opens the chat service over the app's one store, so chats share its lease and its notices. The account's
// model routes are read in the background; the retention sweep is a host job.
func (p *App) attachChats() {
	if p.userStore == nil {
		log.Println("chats: unavailable, the store did not open")
		return
	}
	p.chats = newChatService(p.userStore, p)
	p.builder = p.newBuilder()
	p.chats.RefreshModels()
}

// sweepAIConversations removes the conversations the retention window has passed. The chat store belongs to the UI
// thread, so the sweep runs there, and it does nothing while the palette is open, as the desktop app's sweep does.
func (p *App) sweepAIConversations(ctx context.Context) error {
	done := make(chan error, 1)
	p.onUI(func() {
		if p.Palette != nil && p.Palette.Visible() {
			done <- nil
			return
		}
		removed, err := p.chats.Expire()
		if len(removed) > 0 {
			log.Printf("chats: removed %d idle conversations", len(removed))
		}
		done <- err
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// newChatService wires the chat service to the app: the account's client and sign-in, the UI thread, the spend-cap
// note and the builder's follow-up.
func newChatService(st *store.Store, p *App) *chat.Service {
	return chat.New(chat.Options{
		Store:        chat.OpenStore(st, searchClock),
		Clock:        searchClock,
		Client:       p.backendClient,
		Connected:    p.signedIn,
		Dispatch:     p.onUI,
		NoteError:    p.noteAIError,
		SystemPrompt: p.systemPromptFor,
		Preflight:    p.checkCompatibility,
		OnSettled:    p.chatSettled,
		Tools:        p.chatToolset(),
		BuilderTools: p.builderToolset,
	})
}

// backendClient is the backend client of the active account, or nil when there is none.
func (p *App) backendClient() *backend.Client {
	if p.account == nil {
		return nil
	}
	return p.account.Client()
}

// noteAIError records a spend-cap refusal on the account, so the account view explains it too.
func (p *App) noteAIError(err error) {
	if p.account != nil {
		p.account.NoteError(err)
	}
}

// systemPromptFor is the instruction a chat starts with: the builder writes extensions, a proofread command fixes
// text, and a conversation talks.
func (p *App) systemPromptFor(c chat.Chat) string {
	switch c.Kind {
	case chat.KindBuilder:
		if p.builder != nil {
			return p.builder.SystemPrompt(c)
		}
	case chat.KindProofread:
		return commands.FixSystemPrompt
	}
	return chat.ConversationSystemPrompt
}

// chatSettled runs after a chat's turn ends on the UI thread. The extension builder reacts to its turns.
func (p *App) chatSettled(id string, outcome chat.Outcome) {
	record, ok := p.chats.Chat(id)
	if !ok {
		return
	}
	if record.Kind != chat.KindBuilder {
		return
	}
	if p.builder != nil {
		p.builder.Settled(id, outcome)
	}
	if record.Extension != "" {
	}
}

// drawReply draws a chat reply as Markdown with the app's reply options.
func (p *App) drawReply(c *ui.Context, src string, streaming bool) {
	replyMarkdown(c, src, streaming, &p.md)
}

// startedChatView starts a chat of kind, prompted when prompt is not empty and bound to extension when it is not empty,
// and returns the palette page that shows it. The page opens at once with the built-in's loading view, which fills in
// on a tick, so the caller never waits for the built-in on the UI thread.
func (p *App) startedChatView(kind, prompt, extension string) *palette.View {
	deps := p.extensionViewDeps(p.extensionManager())
	id, err := (extensionChatPages{p: p}).Start(kind, prompt, extension)
	if err != nil {
		log.Println("chat: could not start a chat:", err)
		return extview.NewSession(deps, aichats.ID, api.ErrorView("AI Chat", "The chat store did not open. Restart Nevermind.")).Palette()
	}
	return extview.NewSession(deps, aichats.ID, aichats.LoadingChatView(id)).Palette()
}

// askAI is the Tab page at the root: a conversation that starts with the query.
func (p *App) askAI(q string) *palette.View {
	return p.startedChatView("conversation", q, "")
}
