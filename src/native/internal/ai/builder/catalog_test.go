package builder

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// systemThemeInstalled is the owner's converted extension: an installed toggle whose row is "Toggle System Theme".
var systemThemeInstalled = Installed{ID: "system.toggle-theme", Title: "Toggle System Theme", Enabled: true, Titles: []string{"Toggle System Theme"}}

// duplicateSource is the owner's new extension: the same row title as the installed toggle, under a new id.
const duplicateSource = `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "toggle-theme", Title: "Toggle System Theme"}}, nil
}
`

// TestSystemPromptListsTheInstalledExtensions: a new chat sees what is installed, with its id, state and rows, and the
// rule not to write a duplicate of it.
func TestSystemPromptListsTheInstalledExtensions(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{systemThemeInstalled}
	prompt := New(newFakeConversation(), extensions, func(func()) {}).SystemPrompt(chat.Chat{})
	for _, want := range []string{"system.toggle-theme", `"Toggle System Theme" (on)`, "do not write a duplicate"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not contain %q", want)
		}
	}
}

// TestNewExtensionWithAnInstalledRowTitleIsNotInstalled: a model that writes a second "Toggle System Theme" gets no second
// row. The user is told which extension already has it, and nothing is installed or sent back to the model.
func TestNewExtensionWithAnInstalledRowTitleIsNotInstalled(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], reply("user.system-theme", duplicateSource))
	extensions := newFakeExtensions(checkResult{methods: []string{"commands"}})
	extensions.catalog = []Installed{systemThemeInstalled}
	workshop, _ := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())

	if _, ok := extensions.installed["user.system-theme"]; ok {
		t.Fatal("a duplicate row was installed")
	}
	if len(conversation.sent) != 0 {
		t.Fatalf("the model was sent %q; a title clash asks the user instead", conversation.sent)
	}
	last := conversation.notes[len(conversation.notes)-1]
	if last.Kind != chat.NoticeFailed || !strings.Contains(last.Message, "system.toggle-theme") {
		t.Fatalf("the last note is %+v; want the clash named", last)
	}
}

// TestChatThatEditsItsOwnExtensionKeepsItsRows: the same titles are fine when they belong to the extension the chat
// changes, so an edit is installed.
func TestChatThatEditsItsOwnExtensionKeepsItsRows(t *testing.T) {
	own := Installed{ID: "user.system-theme", Title: "System Theme", Enabled: true, Titles: []string{"Toggle System Theme"}}
	record := builderChat("b1")
	record.Extension = "user.system-theme"
	conversation := newFakeConversation(record)
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], reply("user.system-theme", duplicateSource))
	extensions := newFakeExtensions(checkResult{methods: []string{"commands"}})
	extensions.catalog = []Installed{own}
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if !extensions.enabled["user.system-theme"] {
		t.Fatal("an edit of the chat's own extension was refused for its own row title")
	}
}

// TestReplyThatPointsAtAnInstalledExtensionWritesNothing: a reply with no code that names an installed extension asks the
// user; the chat is not sent back and nothing is installed.
func TestReplyThatPointsAtAnInstalledExtensionWritesNothing(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], "system.toggle-theme already does this. Do you want me to change it?")
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{systemThemeInstalled}
	workshop, _ := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())

	if len(conversation.sent) != 0 || len(extensions.installed) != 0 {
		t.Fatalf("sent %q, installed %v; want nothing written and the user asked", conversation.sent, extensions.installed)
	}
	if last := conversation.notes[len(conversation.notes)-1]; !strings.Contains(last.Message, "system.toggle-theme") {
		t.Fatalf("the last note is %+v; want the existing extension named", last)
	}
}

// TestUnrelatedNoCodeReplyIsStillSentBack: a reply that names nothing installed keeps the bounded fix.
func TestUnrelatedNoCodeReplyIsStillSentBack(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], "Sorry, I cannot.")
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{systemThemeInstalled}
	workshop, _ := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())

	if len(conversation.sent) != 1 {
		t.Fatalf("the model was sent %q; want the block rule", conversation.sent)
	}
}
