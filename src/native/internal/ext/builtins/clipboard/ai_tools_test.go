package clipboard

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func clipboardToolCall(name, args string) api.AIToolCall {
	return api.AIToolCall{Name: name, Args: json.RawMessage(args)}
}

// The history may hold secrets, so the host asks before the AI reads it, whatever a tool declares.
func TestClipboardToolsNeverRunUnasked(t *testing.T) {
	manifest := New(nil).Manifest()
	if err := manifest.ValidateBuiltIn(); err != nil {
		t.Fatal(err)
	}
	for _, tool := range manifest.AITools {
		if class := manifest.EnforcedAIToolClass(tool); class == api.AIToolRead || class == api.AIToolWrite {
			t.Errorf("%s runs as %s", tool.Name, class)
		}
	}
}

func TestSearchClipboardReturnsMatchingTextNewestFirst(t *testing.T) {
	ctx := newFakeContext(text("a", "meeting notes for monday", 1), text("b", "grocery list", 2), text("c", "monday standup", 3))
	result, err := New(nil).CallAITool(ctx, clipboardToolCall(toolSearch, `{"query":"monday"}`))
	entries, _ := result.([]toolEntry)
	if err != nil || len(entries) != 2 || entries[0].ID != "a" || entries[1].Text != "monday standup" {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
}

// A delete is described with the items' first lines from the history, so the question names what goes.
func TestDeletingClipboardItemsIsDescribedFromTheHistory(t *testing.T) {
	ctx := newFakeContext(text("a", "secret token\nsecond line", 1), text("b", "grocery list", 2))
	described, err := New(nil).DescribeAITool(ctx, clipboardToolCall(toolDelete, `{"ids":["a","b"]}`))
	if err != nil || described != "Delete 2 items from clipboard history: “secret token second line”, “grocery list”" {
		t.Fatalf("described = %q, %v", described, err)
	}
	if _, err := New(nil).CallAITool(ctx, clipboardToolCall(toolDelete, `{"ids":["a"]}`)); err != nil || len(ctx.history.removed) != 1 {
		t.Fatalf("err = %v, removed %v", err, ctx.history.removed)
	}
}

func TestAClipboardToolRefusesAnItemTheHistoryNoLongerHas(t *testing.T) {
	ctx := newFakeContext(text("a", "kept", 1))
	_, err := New(nil).DescribeAITool(ctx, clipboardToolCall(toolDelete, `{"ids":["gone"]}`))
	if !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), "search first") {
		t.Fatalf("err = %v", err)
	}
}

func TestCopyClipboardItemPutsItBackOnTheClipboard(t *testing.T) {
	ctx := newFakeContext(text("a", "hello", 1))
	if _, err := New(nil).CallAITool(ctx, clipboardToolCall(toolCopy, `{"id":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).CallAITool(ctx, clipboardToolCall(toolCopy, `{"id":"gone"}`)); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("copying a missing item: err = %v", err)
	}
}
