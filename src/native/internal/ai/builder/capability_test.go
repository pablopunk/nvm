package builder

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// appleScriptSource is the owner's theme toggle written by the model: it runs AppleScript through the shell service.
const appleScriptSource = `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"System Events\"", api.ShellOptions{})
	return api.Result{}, err
}
`

// replyWithManifest is a finished reply whose manifest text is given as written, so a test can omit a capability.
func replyWithManifest(manifest, source string) string {
	return "Here it is.\n\n```json\n" + manifest + "\n```\n\n```go\n" + source + "\n```\n"
}

// TestPromptStatesTheCapabilityRules: the model is told which services need which capability before it writes.
func TestPromptStatesTheCapabilityRules(t *testing.T) {
	prompt := New(newFakeConversation(), newFakeExtensions(), func(func()) {}).SystemPrompt(chat.Chat{})
	for _, want := range []string{
		`ctx.Desktop().Shell() needs "system"`,
		`ctx.Desktop().Shell().OpenExternal() needs "desktop.files"`,
		`ctx.Network() needs "network"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not state %q", want)
		}
	}
}

// TestUndeclaredCapabilityGoesBackToTheModelAndIsFixed is the fake-model loop: the first reply calls Shell().AppleScript
// without declaring system, so it is never checked or installed and the model is told to declare it; the fixed reply
// installs enabled.
func TestUndeclaredCapabilityGoesBackToTheModelAndIsFixed(t *testing.T) {
	bare := `{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme"}`
	declared := `{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme","capabilities":["system"]}`
	record := chat.Chat{ID: "b1", Kind: chat.KindBuilder, Tier: chat.TierSmart, Messages: []chat.Message{
		{Author: chat.AuthorUser, Text: "toggle the theme", Status: chat.StatusComplete},
		{Author: chat.AuthorAssistant, Text: replyWithManifest(bare, appleScriptSource), Status: chat.StatusComplete},
	}}
	conversation := newFakeConversation(record)
	extensions := newFakeExtensions(checkResult{methods: []string{"RunAction"}})
	workshop, queue := newWorkshopFor(conversation, extensions)

	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if extensions.checked != 0 {
		t.Fatalf("the host loaded a source that calls an undeclared service (%d checks)", extensions.checked)
	}
	if len(conversation.sent) != 1 || !strings.Contains(conversation.sent[0], `"system"`) {
		t.Fatalf("the model was sent %q; want the missing system capability", conversation.sent)
	}
	conversation.chats["b1"] = appendReply(conversation.chats["b1"], replyWithManifest(declared, appleScriptSource))
	workshop.Settled("b1", finished())
	awaitUI(t, queue)

	if !extensions.enabled["user.system-theme"] {
		t.Fatal("the fixed extension was not installed enabled")
	}
}
