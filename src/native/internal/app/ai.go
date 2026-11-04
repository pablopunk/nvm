package app

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/search"
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

// chatDeps is what the chat pages draw from and act through.
func (p *App) chatDeps() aichat.Deps {
	return aichat.Deps{
		Service:     p.chats,
		Reply:       p.drawReply,
		Copy:        p.copyChatReply,
		SignedIn:    p.signedIn,
		SignIn:      p.signInState,
		StartSignIn: p.startSignIn,
		Now:         time.Now,
		Review:      p.builderReview,
		PickImages:  p.chooseChatImages,
		PasteImage:  p.clipboardChatImage,
		PreviewRows: p.extensionPreviewRows,
	}
}

// newChatView starts a conversation and returns its page. A first message is sent at once ("Ask AI" from the
// clipboard passes the copied text).
func (p *App) newChatView(first string) *palette.View {
	return p.openChat(chat.KindConversation, chat.TierFast, first)
}

// openChat starts a chat of kind on tier, sends first when it is not empty, and returns its page.
func (p *App) openChat(kind chat.Kind, tier chat.Tier, first string) *palette.View {
	if p.chats == nil {
		return unavailableChatView()
	}
	record, err := p.chats.NewChat(kind, tier)
	if err != nil {
		log.Println("chat: could not start a chat:", err)
		return unavailableChatView()
	}
	if first != "" {
		p.onUI(func() {
			if err := p.chats.Send(record.ID, first); err != nil {
				log.Println("chat: could not send:", err)
			}
		})
	}
	return aichat.ChatView(p.chatDeps(), record.ID)
}

// chatHistoryView lists the saved conversations.
func (p *App) chatHistoryView() *palette.View {
	if p.chats == nil {
		return unavailableChatView()
	}
	return aichat.HistoryView(p.chatDeps())
}

// unavailableChatView is the page shown when the chat store did not open: it says so, and nothing else.
func unavailableChatView() *palette.View {
	return &palette.View{
		Title:       "AI Chat",
		Placeholder: "AI chats are unavailable",
		Search: func(string) []search.Item {
			return []search.Item{{Title: "AI chats are unavailable", Subtitle: "The chat store did not open. Restart Nevermind."}}
		},
	}
}

// chatSettled runs after a chat's turn ends on the UI thread. The extension builder and the AI commands react to theirs.
func (p *App) chatSettled(id string, outcome chat.Outcome) {
	record, ok := p.chats.Chat(id)
	if !ok {
		return
	}
	switch record.Kind {
	case chat.KindBuilder:
		if p.builder != nil {
			p.builder.Settled(id, outcome)
		}
		if record.Extension != "" {
			p.refreshPreviewRows(record.Extension)
		}
	case chat.KindProofread:
		if p.aiCommands != nil {
			p.aiCommands.Settled(id, outcome)
		}
	}
}

// drawReply draws a chat reply as Markdown with the app's reply options.
func (p *App) drawReply(c *ui.Context, src string, streaming bool) {
	replyMarkdown(c, src, streaming, &p.md)
}

// copyChatReply writes a reply to the clipboard. A failure is logged; the palette has already said the copy happened.
func (p *App) copyChatReply(text string) {
	if err := p.ports.Clipboard.WriteText(text); err != nil {
		log.Println("chat: copy failed:", err)
	}
}
