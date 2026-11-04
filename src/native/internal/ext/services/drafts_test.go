package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func decodeDraftLookup(t *testing.T, reply protocol.ServiceReply) protocol.DraftLookup {
	t.Helper()
	var lookup protocol.DraftLookup
	if err := json.Unmarshal(mustOK(t, reply), &lookup); err != nil {
		t.Fatalf("decode draft lookup: %v", err)
	}
	return lookup
}

func saveDraft(t *testing.T, h *Handler, key, version, content string) {
	t.Helper()
	mustOK(t, serve(t, h, protocol.ServiceDrafts, protocol.OpSave, protocol.DraftSaveArgs{Key: key, Version: version, Content: content}))
}

func TestDraftSaveGetAndCommit(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.notes"))
	saveDraft(t, h, "note-1", "v1", "typed before saving")

	lookup := decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "note-1"}))
	if !lookup.Found || lookup.Draft.Content != "typed before saving" || lookup.Draft.Version != "v1" {
		t.Fatalf("lookup = %+v; want the saved draft", lookup)
	}
	if lookup.Draft.SavedAtMs != r.clock.Now().UnixMilli() {
		t.Fatalf("savedAtMs = %d; want the host clock", lookup.Draft.SavedAtMs)
	}

	removed := decodeBool(t, mustOK(t, serve(t, h, protocol.ServiceDrafts, protocol.OpCommit, protocol.DraftCommitArgs{Key: "note-1", Version: "v2"})))
	if !removed {
		t.Fatal("commit did not report the draft it removed")
	}
	if decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "note-1"})).Found {
		t.Fatal("a committed draft is still stored")
	}
	if decodeBool(t, mustOK(t, serve(t, h, protocol.ServiceDrafts, protocol.OpCommit, protocol.DraftCommitArgs{Key: "note-1", Version: "v2"}))) {
		t.Fatal("a second commit reported a removal")
	}
}

func TestDraftSaveReplacesTheDraftOfTheSameKey(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.notes"))
	saveDraft(t, h, "note-1", "v1", "first")
	saveDraft(t, h, "note-1", "v1", "second")
	lookup := decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "note-1"}))
	if lookup.Draft.Content != "second" {
		t.Fatalf("content = %q; want the latest save", lookup.Draft.Content)
	}
}

// A draft is on disk when its save returns: a process killed right after (no Close, no flush) leaves it for the next
// launch. The second handler opens the same folder, as a restarted app does.
func TestDraftSurvivesAKilledProcess(t *testing.T) {
	r := newRig(t)
	killed := handlerFor(t, r, manifestFor("acme.notes"))
	saveDraft(t, killed, "note-1", "v1", "typed right before the kill")

	restarted := handlerFor(t, r, manifestFor("acme.notes"))
	lookup := decodeDraftLookup(t, serve(t, restarted, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "note-1"}))
	if !lookup.Found || lookup.Draft.Content != "typed right before the kill" {
		t.Fatalf("after the kill, lookup = %+v; want the draft", lookup)
	}
}

func TestDraftsAreNamespacedPerExtension(t *testing.T) {
	r := newRig(t)
	saveDraft(t, handlerFor(t, r, manifestFor("acme.one")), "shared-key", "v1", "private")
	other := handlerFor(t, r, manifestFor("acme.two"))
	if decodeDraftLookup(t, serve(t, other, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "shared-key"})).Found {
		t.Fatal("another extension read this extension's draft")
	}
}

func TestDraftDiscardRemovesOnlyItsKey(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.notes"))
	saveDraft(t, h, "a", "v1", "one")
	saveDraft(t, h, "b", "v1", "two")
	mustOK(t, serve(t, h, protocol.ServiceDrafts, protocol.OpDiscard, protocol.KeyArgs{Key: "a"}))
	if decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "a"})).Found {
		t.Fatal("a discarded draft is still stored")
	}
	if !decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "b"})).Found {
		t.Fatal("discarding a removed the draft of b")
	}
}

func TestDraftQuotasAndInvalidInputAreRefused(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.notes"))
	saveDraft(t, h, "keep", "v1", "old text")

	big := strings.Repeat("x", maxDraftBytes+1)
	requireCode(t, serve(t, h, protocol.ServiceDrafts, protocol.OpSave, protocol.DraftSaveArgs{Key: "keep", Version: "v1", Content: big}), api.CodeQuota, "256 KB")
	requireCode(t, serve(t, h, protocol.ServiceDrafts, protocol.OpSave, protocol.DraftSaveArgs{Key: "", Version: "v1"}), api.CodeInvalid, "")
	requireCode(t, serve(t, h, protocol.ServiceDrafts, protocol.OpSave, protocol.DraftSaveArgs{Key: "k", Version: strings.Repeat("v", maxDraftVersionLen+1)}), api.CodeInvalid, "")
	requireCode(t, serve(t, h, protocol.ServiceDrafts, "unknown", protocol.KeyArgs{Key: "k"}), api.CodeInvalid, "")

	lookup := decodeDraftLookup(t, serve(t, h, protocol.ServiceDrafts, protocol.OpGet, protocol.KeyArgs{Key: "keep"}))
	if lookup.Draft.Content != "old text" {
		t.Fatalf("a refused save changed the draft to %q", lookup.Draft.Content)
	}
}

func TestDraftCountIsBounded(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.notes"))
	for i := 0; i < maxDrafts; i++ {
		saveDraft(t, h, keyName(i), "v1", "text")
	}
	requireCode(t, serve(t, h, protocol.ServiceDrafts, protocol.OpSave, protocol.DraftSaveArgs{Key: "one-too-many", Version: "v1"}), api.CodeQuota, "64 drafts")
	saveDraft(t, h, keyName(0), "v2", "replaced")
}

func decodeBool(t *testing.T, raw json.RawMessage) bool {
	t.Helper()
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode bool %s: %v", raw, err)
	}
	return value
}
