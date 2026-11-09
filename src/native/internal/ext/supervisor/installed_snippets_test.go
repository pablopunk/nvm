package supervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func snippetTitles(view api.View) string {
	var titles []string
	for _, item := range view.Items {
		titles = append(titles, item.Title)
	}
	return strings.Join(titles, ",")
}

// TestInstalledSnippetsAddEditAndDeleteKeepTheListInStep runs the installed Text Snippets extension through its whole
// life with real storage: an empty list, two saved snippets newest first, an edit that is refused when the snippet
// changed since it was opened and accepted when it did not, and a delete.
func TestInstalledSnippetsAddEditAndDeleteKeepTheListInStep(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.text-snippets")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	r := servicedRuntime(t, filepath.Join("installed", "local.text-snippets"), rig, installedManifest(t, raw))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	run := func(in api.Invocation) api.View { return runInstalledView(t, ctx, r, in) }

	empty := run(api.Invocation{Ref: api.ActionRef{ID: "snippets"}})
	if len(empty.Items) != 0 || empty.EmptyView == nil || empty.EmptyView.Title != "No snippets yet" {
		t.Fatalf("first list = %+v, want the empty state", empty)
	}

	run(api.Invocation{Ref: api.ActionRef{ID: "save"}, EditorContent: "Hello world\nsecond line"})
	time.Sleep(20 * time.Millisecond)
	two := run(api.Invocation{Ref: api.ActionRef{ID: "save"}, EditorContent: "Another note"})
	if snippetTitles(two) != "Another note,Hello world" {
		t.Fatalf("snippets = %q, want the newest first", snippetTitles(two))
	}
	hello := two.Items[1]
	if hello.Subtitle != "Hello world second line" {
		t.Fatalf("subtitle = %q, want the lines joined", hello.Subtitle)
	}
	if hello.Primary == nil || hello.Primary.Title != "Copy Snippet" || !hello.Primary.Background {
		t.Fatalf("primary = %+v, want a background copy", hello.Primary)
	}

	editor := run(api.Invocation{Ref: api.ActionRef{ID: "edit", Args: map[string]string{"id": hello.ID}}})
	if editor.Kind != api.ViewEditor || editor.Editor == nil || editor.Editor.Content != "Hello world\nsecond line" || editor.Editor.Submit == nil {
		t.Fatalf("editor = %+v, want the snippet's text with a save action", editor)
	}
	save := *editor.Editor.Submit.Ref
	stale := api.ActionRef{ID: save.ID, Args: map[string]string{"id": hello.ID, "updatedAt": "1"}}
	_, err = r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: stale, EditorContent: "overwrite"})
	var callErr *CallError
	if !errors.As(err, &callErr) || !strings.Contains(callErr.Failure.Message, "changed since you opened it") {
		t.Fatalf("saving a stale edit = %v, want it refused", err)
	}
	edited := run(api.Invocation{Ref: save, EditorContent: "Hello again"})
	if snippetTitles(edited) != "Hello again,Another note" {
		t.Fatalf("snippets = %q, want the edited one first", snippetTitles(edited))
	}

	remaining := run(api.Invocation{Ref: api.ActionRef{ID: "delete", Args: map[string]string{"id": edited.Items[0].ID}}})
	if snippetTitles(remaining) != "Another note" {
		t.Fatalf("snippets after delete = %q, want Another note", snippetTitles(remaining))
	}
}
