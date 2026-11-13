package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// bindBuilderChat records that a builder chat wrote extension id, as the workshop does after an install. The chat has
// a notice of the install, so it is recorded.
func bindBuilderChat(t *testing.T, chats *chat.Service, id string) {
	t.Helper()
	record, err := chats.NewChat(chat.KindBuilder, chat.TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	if err := chats.Note(record.ID, chat.Notice{Kind: chat.NoticeNote, Title: "Extension", Message: "Installed."}); err != nil {
		t.Fatal(err)
	}
	if err := chats.Bind(record.ID, id); err != nil {
		t.Fatal(err)
	}
}

func actionTitlesOf(it search.Item) []string {
	titles := make([]string, 0, len(it.Actions))
	for _, action := range it.Actions {
		titles = append(titles, action.Title)
	}
	return titles
}

// TestEditWithAIIsOfferedOnlyForAnExtensionABuilderWrote: the row offers the edit when a builder chat owns the
// extension, and only then.
func TestEditWithAIIsOfferedOnlyForAnExtensionABuilderWrote(t *testing.T) {
	chats := goldenChatService(t, fake.NewSet())
	bindBuilderChat(t, chats, "acme.good")
	p := &App{chats: chats}
	ctl := &fakeExtensions{}

	owned := extensionListRow(p, ctl, goodListing())
	if !hasTitle(actionTitlesOf(owned), "Edit with AI") {
		t.Fatalf("actions = %v; want Edit with AI for a builder's extension", actionTitlesOf(owned))
	}
	other := goodListing()
	other.ID = "acme.other"
	if hasTitle(actionTitlesOf(extensionListRow(p, ctl, other)), "Edit with AI") {
		t.Fatal("Edit with AI is offered for an extension no builder chat wrote")
	}
	if hasTitle(actionTitlesOf(extensionListRow(&App{}, ctl, goodListing())), "Edit with AI") {
		t.Fatal("Edit with AI is offered without a chat service")
	}
}

// TestEditWithAIStartsAChatBoundToTheExtension: the action opens a builder chat that the extension is bound to, so the
// next reply changes that extension. The opened chat is recorded with its first message, not before.
func TestEditWithAIStartsAChatBoundToTheExtension(t *testing.T) {
	chats := goldenChatService(t, fake.NewSet())
	bindBuilderChat(t, chats, "acme.good")
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{chats: chats}
	p.exts.mgr.Store(m)
	row := extensionListRow(p, &fakeExtensions{}, goodListing())
	h := &recordingHost{}
	actionTitled(t, row, "Edit with AI").Run(h)

	if len(h.pushed) != 1 {
		t.Fatalf("pushed %d views; want the chat view", len(h.pushed))
	}
	if owners := chats.Owners("acme.good"); len(owners) != 1 {
		t.Fatalf("owners of acme.good = %d chats; want only the recorded original, since an opened edit has no message yet", len(owners))
	}
}

// TestEditWithAINamesTheChatTweakOfTheExtension: the chat an edit opens is titled Tweak and the extension's title, as
// 1.x titles its tweak chats, before any message is sent.
func TestEditWithAINamesTheChatTweakOfTheExtension(t *testing.T) {
	chats := goldenChatService(t, fake.NewSet())
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{chats: chats}
	p.exts.mgr.Store(m)

	id, err := extensionChatPages{p: p}.Start("builder", "", "acme.good")
	if err != nil {
		t.Fatal(err)
	}
	record, ok := chats.Chat(id)
	if !ok || record.Title != "Tweak Good fixture" {
		t.Fatalf("the edit chat is titled %q (found %v); want Tweak Good fixture", record.Title, ok)
	}
}
