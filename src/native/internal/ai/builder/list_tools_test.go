package builder

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

func runTool(t *testing.T, w *Workshop, chatID, name, args string) (string, error) {
	t.Helper()
	tool, ok := w.Toolset(chatID).Registry.Lookup(name)
	if !ok {
		t.Fatalf("tool %s is not registered", name)
	}
	result, err := tool.Run(context.Background(), json.RawMessage(args))
	return result.Text, err
}

func TestListExtensionsIsSortedAndPaged(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.catalog = []Installed{{ID: "user.b", Title: "B", Enabled: true, Titles: []string{"Beta"}}, {ID: "user.a", Title: "A"}}
	w := New(newFakeConversation(), extensions, synchronous)
	text, err := runTool(t, w, "c", listExtensionsToolName, `{}`)
	if err != nil || strings.Index(text, "user.a") > strings.Index(text, "user.b") || !strings.Contains(text, "(on)") {
		t.Fatalf("list = %q, %v", text, err)
	}
	for _, bad := range []string{`{"offset":-1}`, `{"offset":100000}`} {
		if _, err := runTool(t, w, "c", listExtensionsToolName, bad); err == nil {
			t.Fatalf("%s accepted", bad)
		}
	}
	extensions.catalog = nil
	if text, _ := runTool(t, w, "c", listExtensionsToolName, `{}`); !strings.Contains(text, "No extensions") {
		t.Fatalf("empty = %q", text)
	}
}

func TestListExtensionsPageIsBounded(t *testing.T) {
	var many []Installed
	for i := 0; i < 2000; i++ {
		many = append(many, Installed{ID: "user.e" + strings.Repeat("x", 20), Title: "T"})
	}
	result, err := listExtensionsPage(many, 0)
	if err != nil || len(result.Text) > apiPageBytes+200 || !strings.Contains(result.Text, "Continue with offset") {
		t.Fatalf("len=%d err=%v", len(result.Text), err)
	}
}

func TestReadCurrentExtensionNeedsABoundChat(t *testing.T) {
	conversation := newFakeConversation(builderChat("free"), chat.Chat{ID: "bound", Kind: chat.KindBuilder, Extension: "user.mine"})
	extensions := newFakeExtensions()
	extensions.files["user.mine"] = map[string][]byte{"main.go": []byte("package main // mine")}
	w := New(conversation, extensions, synchronous)
	if text, err := runTool(t, w, "free", readCurrentExtensionToolName, `{}`); err != nil || !strings.Contains(text, "no extension yet") {
		t.Fatalf("free chat = %q, %v", text, err)
	}
	if _, err := runTool(t, w, "gone", readCurrentExtensionToolName, `{}`); err == nil {
		t.Fatal("unknown chat read an extension")
	}
	if text, err := runTool(t, w, "bound", readCurrentExtensionToolName, `{}`); err != nil || !strings.Contains(text, "package main // mine") {
		t.Fatalf("bound = %q, %v", text, err)
	}
}

func TestReadCurrentExtensionPagesAreSequentialAndBounded(t *testing.T) {
	conversation := newFakeConversation(chat.Chat{ID: "bound", Kind: chat.KindBuilder, Extension: "user.mine"})
	extensions := newFakeExtensions()
	extensions.files["user.mine"] = map[string][]byte{"main.go": []byte(strings.Repeat("x\n", apiPageBytes*2))}
	w := New(conversation, extensions, synchronous)
	toolset := w.Toolset("bound")
	tool, _ := toolset.Registry.Lookup(readCurrentExtensionToolName)
	first, err := tool.Run(context.Background(), json.RawMessage(`{}`))
	if err != nil || len(first.Text) > apiPageBytes+300 {
		t.Fatalf("first page len=%d err=%v", len(first.Text), err)
	}
	if _, err := tool.Run(context.Background(), json.RawMessage(`{"offset":999999}`)); err == nil {
		t.Fatal("skipping pages was accepted")
	}
}

func TestReadToolsRefuseIdsOutsideExtensionSources(t *testing.T) {
	w := New(newFakeConversation(builderChat("c")), newFakeExtensions(), synchronous)
	for _, id := range []string{"../../etc/passwd", "/etc/passwd", "user.missing"} {
		args, _ := json.Marshal(map[string]string{"id": id})
		if _, err := runTool(t, w, "c", readExtensionToolName, string(args)); err == nil {
			t.Fatalf("read of %q succeeded", id)
		}
	}
}

func TestBuilderToolsetHasNoLogOrShortcutReadersAndWritesOnlyThroughWriteExtension(t *testing.T) {
	registry := New(newFakeConversation(builderChat("c")), newFakeExtensions(), synchronous).Toolset("c").Registry
	for _, name := range []string{"read_app_logs", "list_shortcuts", "remove_extension", "install_extension"} {
		if _, ok := registry.Lookup(name); ok {
			t.Fatalf("%s must not be registered", name)
		}
	}
	for _, name := range []string{listExtensionsToolName, readCurrentExtensionToolName, readExtensionToolName, writeToolName} {
		if _, ok := registry.Lookup(name); !ok {
			t.Fatalf("%s missing", name)
		}
	}
}

func TestReadingCurrentExtensionDoesNotMakeAnotherExtensionWritable(t *testing.T) {
	conversation := newFakeConversation(chat.Chat{ID: "bound", Kind: chat.KindBuilder, Extension: "user.mine"})
	w := New(conversation, newFakeExtensions(), synchronous)
	if problem := w.ownershipProblem("user.mine", "user.other"); problem == "" {
		t.Fatal("write to another extension allowed")
	}
}
