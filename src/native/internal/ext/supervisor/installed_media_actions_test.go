package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestInstalledMediaActionsHubListsTheFourActionsForTheFinderSelection runs the installed Media Actions extension against
// a Finder selection of an image, a video and a document: the hub says where the files came from and lists Compress,
// Convert, Resize and Speed Up, each counting how many of the selected files it supports.
func TestInstalledMediaActionsHubListsTheFourActionsForTheFinderSelection(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.media-actions")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Selection = finderSelection{files: []string{"/Users/p/Desktop/a.png", "/Users/p/Desktop/b.mov", "/Users/p/Desktop/c.txt"}}
	r := servicedRuntime(t, filepath.Join("installed", "local.media-actions"), rig, installedManifest(t, raw))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "media-actions"}})
	if !strings.HasPrefix(view.Subtitle, "3 Finder files selected") || !strings.HasSuffix(view.Subtitle, "originals are preserved") {
		t.Fatalf("subtitle = %q, want the Finder selection named and originals preserved", view.Subtitle)
	}
	var titles []string
	supported := map[string]string{}
	for _, item := range view.Items {
		titles = append(titles, item.Title)
		if len(item.Accessories) == 1 {
			supported[item.ID] = item.Accessories[0].Text
		}
		if item.Primary == nil {
			t.Errorf("row %q has no main action", item.Title)
		}
	}
	want := "Compress Media,Convert to Compatible Format,Resize Images to Half (50%),Speed Up Media"
	if strings.Join(titles, ",") != want {
		t.Fatalf("rows = %v, want %s", titles, want)
	}
	if supported["half"] != "1 supported" || supported["speed"] != "1 supported" {
		t.Fatalf("supported counts = %v, want the one image for Resize and the one video for Speed Up", supported)
	}
}
