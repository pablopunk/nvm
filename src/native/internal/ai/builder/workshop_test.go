package builder

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestFinishedReplyInstallsOffAndBindsTheChat is the happy path: the candidate loads, is installed disabled, the chat
// owns the extension, and the user is told to review it.
func TestFinishedReplyInstallsOffAndBindsTheChat(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	extensions := newFakeExtensions(checkResult{methods: []string{"RootItems"}})
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if extensions.enabled["user.clock"] {
		t.Fatal("the extension was installed on; it must stay off until it is reviewed")
	}
	if conversation.bound["b1"] != "user.clock" {
		t.Fatalf("the chat is bound to %q; want user.clock", conversation.bound["b1"])
	}
	if len(conversation.sent) != 0 {
		t.Fatalf("a good reply sent %q back to the model", conversation.sent)
	}
	last := conversation.notes[len(conversation.notes)-1]
	if last.Kind != chat.NoticeNote || !strings.Contains(last.Message, "installed but off") {
		t.Fatalf("the last note is %+v; want the review notice", last)
	}
}

// TestRejectedSourceGoesBackToTheModelAndIsFixed scripts the bad-source reply the rig journey uses: the first source
// does not compile, the model is told why, and its second source installs.
func TestRejectedSourceGoesBackToTheModelAndIsFixed(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	extensions := newFakeExtensions(
		checkResult{err: errors.New("extension source does not compile: undefined: api.Itme")},
		checkResult{methods: []string{"RootItems"}},
	)
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)
	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], "undefined: api.Itme") {
		t.Fatalf("the model was sent %q; want the compile error", conversation.sent)
	}
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], reply("user.clock", goodSource))
	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if extensions.enabled["user.clock"] || conversation.bound["b1"] != "user.clock" {
		t.Fatal("the fixed extension was not installed off and bound to the chat")
	}
}

// TestFixAttemptsAreBounded: after MaxFixAttempts fixes the chat fails with the reason and nothing is installed.
func TestFixAttemptsAreBounded(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	broken := checkResult{err: errors.New("extension source does not compile")}
	extensions := newFakeExtensions(broken)
	workshop, queue := newWorkshopFor(conversation, extensions)

	for range MaxFixAttempts + 1 {
		workshop.Settled("b1", finished())
		awaitUI(t, queue)
		conversation.chats["b1"] = appendReply(conversation.chats["b1"], reply("user.clock", goodSource))
	}

	if len(conversation.sent) != MaxFixAttempts {
		t.Fatalf("the model was asked %d times; want %d", len(conversation.sent), MaxFixAttempts)
	}
	last := conversation.notes[len(conversation.notes)-1]
	if last.Kind != chat.NoticeFailed || !strings.Contains(last.Message, "does not compile") {
		t.Fatalf("the last note is %+v; want the failure with its reason", last)
	}
	if _, ok := extensions.installed["user.clock"]; ok {
		t.Fatal("a rejected extension was installed")
	}
}

// TestReplyWithoutBlocksIsSentBack covers a reply that forgot the code: it is a bounded fix, not a silent stop.
func TestReplyWithoutBlocksIsSentBack(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], "Sorry, I cannot.")
	workshop, _ := newWorkshopFor(conversation, newFakeExtensions())

	workshop.Settled("b1", finished())

	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], "```go block") {
		t.Fatalf("the model was sent %q; want the block rule", conversation.sent)
	}
}

// TestSourceWithoutEntryPointIsRefused: a source that loads but implements nothing is not an extension.
func TestSourceWithoutEntryPointIsRefused(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	extensions := newFakeExtensions(checkResult{methods: nil})
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], "no entry point") {
		t.Fatalf("the model was sent %q; want the entry point rule", conversation.sent)
	}
}

// TestStoppedReplyIsLeftAlone: only a finished reply is built.
func TestStoppedReplyIsLeftAlone(t *testing.T) {
	conversation := newFakeConversation(builderChat("b1"))
	workshop, queue := newWorkshopFor(conversation, newFakeExtensions())

	workshop.Settled("b1", chat.Outcome{Ending: chat.EndStopped})

	select {
	case <-queue:
		t.Fatal("a stopped reply was built")
	default:
	}
	if len(conversation.sent) != 0 || len(conversation.notes) != 0 {
		t.Fatal("a stopped reply wrote to the chat")
	}
}

// TestNewIDMustNotBelongToAnotherChatOrExtension protects other extensions and other chats' work.
func TestNewIDMustNotBelongToAnotherChatOrExtension(t *testing.T) {
	owner := builderChat("other")
	owner.Extension = "user.clock"
	conversation := newFakeConversation(builderChat("b1"), owner)
	extensions := newFakeExtensions()
	extensions.existing["user.clock"] = api.Manifest{Identity: api.Identity{ID: "user.clock", Title: "Clock"}}
	workshop, _ := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())

	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], "belongs to another AI chat") {
		t.Fatalf("the model was sent %q; want the ownership rule", conversation.sent)
	}
}

// TestEditedChatKeepsItsExtensionID: a chat bound to an extension may not write a different id.
func TestEditedChatKeepsItsExtensionID(t *testing.T) {
	edit := builderChat("b1")
	edit.Extension = "user.clock"
	conversation := newFakeConversation(edit)
	workshop, _ := newWorkshopFor(conversation, newFakeExtensions())
	conversation.chats["b1"] = appendReply(edit, reply("user.other", goodSource))

	workshop.Settled("b1", finished())

	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], "Keep its id") {
		t.Fatalf("the model was sent %q; want the id rule", conversation.sent)
	}
}

// TestEditingReplacesTheInstalledVersionOff: an edit of an enabled extension installs a new version that is off.
func TestEditingReplacesTheInstalledVersionOff(t *testing.T) {
	edit := builderChat("b1")
	edit.Extension = "user.clock"
	conversation := newFakeConversation(edit)
	extensions := newFakeExtensions(checkResult{methods: []string{"RootItems"}})
	extensions.existing["user.clock"] = api.Manifest{Identity: api.Identity{ID: "user.clock", Title: "Clock"}}
	extensions.enabled["user.clock"] = true
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if extensions.enabled["user.clock"] {
		t.Fatal("the edited extension stayed on without a review")
	}
}

// TestSystemPromptCarriesTheCurrentFilesOfAnEdit checks the edit context names the id and includes the source.
func TestSystemPromptCarriesTheCurrentFilesOfAnEdit(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.existing["user.clock"] = api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "user.clock", Title: "Clock"}}
	extensions.source["user.clock"] = "package main // current marker\n"
	workshop, _ := newWorkshopFor(newFakeConversation(), extensions)

	prompt := workshop.SystemPrompt(chat.Chat{ID: "b1", Kind: chat.KindBuilder, Extension: "user.clock"})

	if !strings.Contains(prompt, "current marker") || !strings.Contains(prompt, `Its id is "user.clock"`) {
		t.Fatal("the edit prompt does not carry the extension's current source and id")
	}
	if strings.Contains(workshop.SystemPrompt(chat.Chat{Kind: chat.KindBuilder}), "current marker") {
		t.Fatal("a new extension's prompt carries another extension's source")
	}
}

func appendReply(record chat.Chat, text string) chat.Chat {
	next := record
	next.Messages = append(append([]chat.Message(nil), record.Messages...), chat.Message{Author: chat.AuthorAssistant, Text: text, Status: chat.StatusComplete})
	return next
}
