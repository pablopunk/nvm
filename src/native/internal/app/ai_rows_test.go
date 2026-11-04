package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestRootContinueRowsIncludeBuilderChatsButNotProofreads checks the root offers the builder chats beside the
// conversations, with the names the desktop app uses, and never a proofread.
func TestRootContinueRowsIncludeBuilderChatsButNotProofreads(t *testing.T) {
	chats := goldenChatService(t, fake.NewSet())
	builder, err := chats.NewChat(chat.KindBuilder, chat.TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	if err := chats.Send(builder.ID, "Build a clock"); err != nil {
		t.Fatal(err)
	}
	proofread, err := chats.NewChat(chat.KindProofread, chat.TierFast)
	if err != nil {
		t.Fatal(err)
	}
	if err := chats.Send(proofread.ID, "Fix the grammar"); err != nil {
		t.Fatal(err)
	}
	p := &App{chats: chats}
	rows := conversationRows(p, &rootInput{chats: p.chatHeads()}, "")
	titles := rowTitlesOf(rows)
	if !hasTitle(titles, "Continue AI chat: Build a clock") {
		t.Fatalf("root rows = %v, want the builder chat as Continue AI chat", titles)
	}
	if !hasTitle(titles, "Continue conversation: Release notes") {
		t.Fatalf("root rows = %v, want the conversation as Continue conversation", titles)
	}
	if hasTitle(titles, "Continue AI chat: Fix the grammar") || hasTitle(titles, "Continue conversation: Fix the grammar") {
		t.Fatalf("root rows = %v, must not offer a proofread", titles)
	}
}
