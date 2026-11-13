package builder

import (
	"context"
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

// systemThemeManifest is the manifest of a new extension under the id user.system-theme.
const systemThemeManifest = `{"apiVersion":"1.0","id":"user.system-theme","title":"System Theme"}`

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

// TestNewExtensionWithAnInstalledRowTitleIsRefused: a model that writes a second "Toggle System Theme" gets no second
// row. The write is refused as the tool's error, naming the extension that has the row, and nothing is installed.
func TestNewExtensionWithAnInstalledRowTitleIsRefused(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{systemThemeInstalled}
	w := New(newFakeConversation(builderChat("b1")), extensions, synchronous)

	_, err := writeForTest(w, context.Background(), "b1", writeArgsFor(t, systemThemeManifest, duplicateSource))
	if err == nil || !strings.Contains(err.Error(), "system.toggle-theme") {
		t.Fatalf("err = %v, want the row clash naming system.toggle-theme", err)
	}
	if len(extensions.installs) != 0 {
		t.Fatalf("installs = %v, want none", extensions.installs)
	}
}

// TestChatThatEditsItsOwnExtensionKeepsItsRows: the same titles are fine when they belong to the extension the chat
// changes, so an edit is installed.
func TestChatThatEditsItsOwnExtensionKeepsItsRows(t *testing.T) {
	own := Installed{ID: "user.system-theme", Title: "System Theme", Enabled: true, Titles: []string{"Toggle System Theme"}}
	record := builderChat("b1")
	record.Extension = "user.system-theme"
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{own}
	w := New(newFakeConversation(record), extensions, synchronous)

	if _, err := writeForTest(w, context.Background(), "b1", writeArgsFor(t, systemThemeManifest, duplicateSource)); err != nil {
		t.Fatalf("an edit of the chat's own extension was refused: %v", err)
	}
	if strings.Join(extensions.installs, ",") != "user.system-theme" {
		t.Fatalf("installs = %v, want user.system-theme", extensions.installs)
	}
}
