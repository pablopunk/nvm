package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestDownloadsTrashRemovesTheRowAndInvalidatesItsCachedViews runs the Downloads Big Files extension's trash action
// against a scripted Finder: the row is removed from the open list, and the extension asks the host to drop its cached
// rows so that reopening the list does not show the trashed file again.
func TestDownloadsTrashRemovesTheRowAndInvalidatesItsCachedViews(t *testing.T) {
	requireUnixProcesses(t)
	commands := fake.NewCommands()
	commands.Script("osascript", fake.HelperScript{})
	var mu sync.Mutex
	var invalidated []string
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	rig.deps.OS = "darwin"
	rig.deps.ViewsInvalidate = func(id string) {
		mu.Lock()
		defer mu.Unlock()
		invalidated = append(invalidated, id)
	}
	r := servicedRuntime(t, "local.downloads-big-files", rig, fixtureManifest("local.downloads-big-files", api.CapabilityDesktopFiles, api.CapabilitySystem))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := filepath.Join(rig.deps.Home, "Downloads", "movie.mkv")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	result := runInstalledResult(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "move-to-trash", Args: map[string]string{"path": path}}})
	if result.Patch == nil || len(result.Patch.RemoveItemIDs) != 1 || result.Patch.RemoveItemIDs[0] != path {
		t.Fatalf("patch = %+v, want the trashed file's row removed", result.Patch)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), invalidated...)
		mu.Unlock()
		if len(got) == 1 && got[0] == "local.downloads-big-files" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("invalidated = %v, want the extension's cached views dropped once", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
