package builder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// clockArgs is the write_extension arguments of a valid clock extension.
func clockArgs(t *testing.T) json.RawMessage {
	t.Helper()
	args, err := json.Marshal(map[string]any{
		"manifest": `{"apiVersion":"1.0","id":"user.clock","title":"Clock"}`,
		"source":   "package main\n\nfunc SearchItems() {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	return args
}

// synchronous runs every UI call at once, as a UI thread that is free does.
func synchronous(fn func()) { fn() }

// TestWriteInstallsEnabledAndBindsTheChatWhenTheTurnSettles checks the write: the extension is checked, installed
// enabled, the steps the chat shows are checking then installing and then cleared, and the chat is bound to the
// extension once its turn settles.
func TestWriteInstallsEnabledAndBindsTheChatWhenTheTurnSettles(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := newFakeExtensions()
	w := New(conversation, extensions, synchronous)

	result, err := w.writeExtension(context.Background(), "chat-1", clockArgs(t))
	if err != nil {
		t.Fatalf("writeExtension: %v", err)
	}
	if !strings.Contains(result.Text, `Installed and enabled "Clock"`) || result.Outcome != "Installed and enabled" {
		t.Fatalf("result = %+v, want the installed line", result)
	}
	if strings.Join(extensions.installs, ",") != "user.clock" {
		t.Fatalf("installs = %v, want user.clock", extensions.installs)
	}
	if strings.Join(conversation.steps, "|") != "Checking…|Installing…|" {
		t.Fatalf("steps = %q, want checking, installing, then cleared", conversation.steps)
	}
	if _, bound := conversation.bound["chat-1"]; bound {
		t.Fatal("the chat was bound before its turn settled")
	}
	w.Settled("chat-1", finished())
	if conversation.bound["chat-1"] != "user.clock" {
		t.Fatalf("bound = %v, want the chat bound to user.clock", conversation.bound)
	}
}

// TestWriteReturnsARejectionAndInstallsNothing checks an extension without an entry point is refused as the tool's
// error, so the model can fix it in the same turn, and nothing is installed.
func TestWriteReturnsARejectionAndInstallsNothing(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := newFakeExtensions()
	extensions.methods = nil
	w := New(conversation, extensions, synchronous)

	_, err := w.writeExtension(context.Background(), "chat-1", clockArgs(t))
	if err == nil || !strings.Contains(err.Error(), "no entry point") {
		t.Fatalf("err = %v, want the entry point rejection", err)
	}
	if len(extensions.installs) != 0 || conversation.steps[len(conversation.steps)-1] != "" {
		t.Fatalf("installs = %v, steps = %q; want nothing installed and the step cleared", extensions.installs, conversation.steps)
	}
}

// TestWriteRefusesAnIdAnotherExtensionHas checks a new chat cannot take the id of an installed extension.
func TestWriteRefusesAnIdAnotherExtensionHas(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	extensions := newFakeExtensions()
	extensions.existing["user.clock"] = clockManifestValue(t)
	w := New(conversation, extensions, synchronous)

	_, err := w.writeExtension(context.Background(), "chat-1", clockArgs(t))
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want the id refusal", err)
	}
	if len(extensions.installs) != 0 {
		t.Fatalf("installs = %v, want none", extensions.installs)
	}
}

// TestBoundChatKeepsItsIdAndRefusesAnother checks a chat that changes an extension may only write that extension's id.
func TestBoundChatKeepsItsIdAndRefusesAnother(t *testing.T) {
	record := builderChat("chat-1")
	record.Extension = "user.clock"
	conversation := newFakeConversation(record)
	extensions := newFakeExtensions()
	w := New(conversation, extensions, synchronous)

	other, _ := json.Marshal(map[string]any{"manifest": `{"apiVersion":"1.0","id":"user.other","title":"Other"}`, "source": "package main\n"})
	if _, err := w.writeExtension(context.Background(), "chat-1", other); err == nil || !strings.Contains(err.Error(), "Keep its id") {
		t.Fatalf("err = %v, want the bound-id refusal", err)
	}
	if _, err := w.writeExtension(context.Background(), "chat-1", clockArgs(t)); err != nil {
		t.Fatalf("the bound id should write: %v", err)
	}
}

// TestWriteStopsWaitingWhenTheTurnEnds checks a write whose turn is cancelled before the UI thread answers returns
// rather than waiting forever.
func TestWriteStopsWaitingWhenTheTurnEnds(t *testing.T) {
	conversation := newFakeConversation(builderChat("chat-1"))
	w := New(conversation, newFakeExtensions(), func(func()) {})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.writeExtension(ctx, "chat-1", clockArgs(t)); err == nil {
		t.Fatal("a write whose turn ended should fail, not wait")
	}
}

// TestWriteFailureFromTheInstallIsReturned checks an install that the manager refuses is the tool's error.
func TestWriteFailureFromTheInstallIsReturned(t *testing.T) {
	extensions := newFakeExtensions()
	extensions.installErr = errors.New("disk full")
	w := New(newFakeConversation(builderChat("chat-1")), extensions, synchronous)
	if _, err := w.writeExtension(context.Background(), "chat-1", clockArgs(t)); err == nil || err.Error() != "disk full" {
		t.Fatalf("err = %v, want the install failure", err)
	}
}

// TestWriteArgumentsAreCheckedFirst checks a write with a bad manifest never reaches the chat or the manager.
func TestWriteArgumentsAreCheckedFirst(t *testing.T) {
	extensions := newFakeExtensions()
	conversation := newFakeConversation(builderChat("chat-1"))
	w := New(conversation, extensions, synchronous)
	bad, _ := json.Marshal(map[string]any{"manifest": "{", "source": "package main"})
	if _, err := w.writeExtension(context.Background(), "chat-1", bad); err == nil || len(conversation.steps) != 0 {
		t.Fatalf("err = %v, steps = %q; want a refusal before any step", err, conversation.steps)
	}
}

// TestToolsetOffersTheReaderAndTheWriter checks a builder turn is offered exactly the two tools.
func TestToolsetOffersTheReaderAndTheWriter(t *testing.T) {
	w := New(newFakeConversation(), newFakeExtensions(), synchronous)
	toolset := w.Toolset("chat-1")
	wire, err := toolset.Registry.Wire()
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	for _, name := range []string{readAPIToolName, writeToolName} {
		if !strings.Contains(string(wire), name) {
			t.Fatalf("tools = %s, want %s", wire, name)
		}
	}
}

// TestSummaryOfAWriteIsTheExtensionId checks the transcript row names the extension, not the source it carries.
func TestSummaryOfAWriteIsTheExtensionId(t *testing.T) {
	if got := summarizeWrite(clockArgs(t)); got != "user.clock" {
		t.Fatalf("summary = %q, want user.clock", got)
	}
}

// clockManifestValue is the manifest of the clock extension as the manager returns it.
func clockManifestValue(t *testing.T) api.Manifest {
	t.Helper()
	var manifest api.Manifest
	if err := json.Unmarshal([]byte(`{"apiVersion":"1.0","id":"user.clock","title":"Clock"}`), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}
