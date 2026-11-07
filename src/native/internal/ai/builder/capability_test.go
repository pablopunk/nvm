package builder

import (
	"context"
	"encoding/json"
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

// writeArgsFor returns the write_extension arguments of one manifest text and one main source.
func writeArgsFor(t *testing.T, manifest, source string) json.RawMessage {
	t.Helper()
	args, err := json.Marshal(map[string]any{"manifest": manifest, "source": source})
	if err != nil {
		t.Fatal(err)
	}
	return args
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

// TestUndeclaredCapabilityIsRefusedThenInstalledWhenDeclared: the write that calls Shell().AppleScript without declaring
// system is refused as the tool's error and installs nothing; the same source with system declared installs.
func TestUndeclaredCapabilityIsRefusedThenInstalledWhenDeclared(t *testing.T) {
	bare := `{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme"}`
	declared := `{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme","capabilities":["system"]}`
	extensions := newFakeExtensions()
	w := New(newFakeConversation(builderChat("b1")), extensions, synchronous)

	_, err := w.writeExtension(context.Background(), "b1", writeArgsFor(t, bare, appleScriptSource))
	if err == nil || !strings.Contains(err.Error(), `"system"`) {
		t.Fatalf("err = %v, want the missing system capability", err)
	}
	if len(extensions.installs) != 0 {
		t.Fatalf("installs = %v, want none after the refusal", extensions.installs)
	}
	if _, err := w.writeExtension(context.Background(), "b1", writeArgsFor(t, declared, appleScriptSource)); err != nil {
		t.Fatalf("the declared extension should write: %v", err)
	}
	if strings.Join(extensions.installs, ",") != "user.system-theme" {
		t.Fatalf("installs = %v, want user.system-theme", extensions.installs)
	}
}
