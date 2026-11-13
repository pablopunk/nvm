package supervisor

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// fixedFileIndex answers the index with one note, so each index method of the fixture has a known answer.
type fixedFileIndex struct{ root string }

func (f fixedFileIndex) Roots() []string { return []string{f.root} }

func (f fixedFileIndex) Search(context.Context, files.Query) []files.Result {
	return []files.Result{{Path: f.root + "/notes/todo.txt", Name: "todo.txt", Kind: files.KindFile}}
}

func (fixedFileIndex) Rebuild() int { return 1 }

func (fixedFileIndex) Status() files.Status { return files.Status{Ready: true, Entries: 1} }

// TestFilesFixtureUsesEveryFilesMethodThroughTheChild runs the files fixture in its child process. Each row shows what the
// host answered for one method of ctx.desktop.files, and the quick look reaches the shell.
func TestFilesFixtureUsesEveryFilesMethodThroughTheChild(t *testing.T) {
	rig := newServicesRig(t)
	rig.deps.FileIndex = fixedFileIndex{root: rig.root}
	rig.deps.Shell = fake.NewShell()
	rig.deps.Capabilities = platform.NewCapabilities(platform.CapabilityShellQuickLook)
	if err := rig.deps.Files.MkdirAll(rig.root+"/notes", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rig.deps.Files.WriteFile(rig.root+"/notes/todo.txt", []byte("buy milk"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := servicedRuntime(t, "acme.files", rig, fixtureManifest("acme.files", api.CapabilityDesktopFiles, api.CapabilitySystem))

	want := map[string]string{
		"recent":    "recent count=1 first=todo.txt",
		"search":    "search count=1 first=todo.txt",
		"snapshot":  "snapshot images=1",
		"roots":     "roots count=1",
		"reindex":   "reindex count=1",
		"text":      "text buy milk",
		"write":     "write out.txt",
		"quicklook": "quicklook shown=true",
		"pictures":  "pictures count=0",
	}
	for query, title := range want {
		if got := rowTitle(t, r, query); got != title {
			t.Errorf("%s row = %q, want %q", query, got, title)
		}
	}
}
