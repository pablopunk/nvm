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

// finderSelection is a selection engine whose Finder selection is a fixed list of files.
type finderSelection struct{ files []string }

func (finderSelection) Text() (string, bool, error) { return "", false, nil }
func (finderSelection) Replace(string) (bool, error) { return false, nil }
func (finderSelection) Undo() error                       { return nil }
func (s finderSelection) Files(context.Context) ([]string, error) { return s.files, nil }

func convertRuntime(t *testing.T, files []string) *Runtime {
	t.Helper()
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.convert-to-compatible")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Selection = finderSelection{files: files}
	return servicedRuntime(t, filepath.Join("installed", "local.convert-to-compatible"), rig, installedManifest(t, raw))
}

// TestInstalledConvertListsOnlyTheSelectedMedia runs the installed Convert to Compatible Format extension against a
// Finder selection of an image, a video and a document: the list shows the two media files with the format each will
// become, and offers Convert & Compress All as the main action of every row.
func TestInstalledConvertListsOnlyTheSelectedMedia(t *testing.T) {
	requireUnixProcesses(t)
	r := convertRuntime(t, []string{"/Users/p/Desktop/photo.PNG", "/Users/p/Desktop/clip.mov", "/Users/p/Desktop/notes.txt"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "convert-media-compatible"}})
	if view.Subtitle != "2 selected · images → JPG, videos → MP4" {
		t.Fatalf("subtitle = %q, want the two media files counted", view.Subtitle)
	}
	var rows []string
	for _, item := range view.Items {
		rows = append(rows, item.Title+" | "+item.Subtitle)
		if item.Primary == nil || item.Primary.Title != "Convert & Compress All" {
			t.Errorf("row %q primary = %+v, want Convert & Compress All", item.Title, item.Primary)
		}
	}
	want := "photo.PNG | PNG → JPG (compressed),clip.mov | MOV → MP4 (compressed)"
	if strings.Join(rows, ",") != want {
		t.Fatalf("rows = %v, want %s", rows, want)
	}
}

// TestInstalledConvertExplainsAnEmptySelection checks that nothing selected, or no media selected, opens a plain
// explanation instead of an empty list.
func TestInstalledConvertExplainsAnEmptySelection(t *testing.T) {
	requireUnixProcesses(t)
	r := convertRuntime(t, []string{"/Users/p/Desktop/notes.txt"})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "convert-media-compatible"}})
	if len(view.Items) != 1 || view.Items[0].Title != "No Media Selected" {
		t.Fatalf("items = %+v, want the No Media Selected explanation", view.Items)
	}
}
