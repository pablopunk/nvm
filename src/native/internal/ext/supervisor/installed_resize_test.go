package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestInstalledResizeShowsEachImageWithItsHalfSize runs the installed Resize Image to Half extension against a Finder
// selection of one image and one document, with sips reporting the image's size: the list shows the image only, with
// its size and the half size it will become, and its file size.
func TestInstalledResizeShowsEachImageWithItsHalfSize(t *testing.T) {
	requireUnixProcesses(t)
	home := t.TempDir()
	image := filepath.Join(home, "photo.png")
	if err := os.WriteFile(image, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.resize-image-half")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	commands := fake.NewCommands()
	commands.Script("sips", fake.HelperScript{Stdout: []byte(image + "\n  pixelWidth: 4000\n  pixelHeight: 3000\n")})
	rig := newServicesRig(t)
	rig.deps.Home = home
	rig.deps.Commands = commands
	rig.deps.Selection = finderSelection{files: []string{image, filepath.Join(home, "notes.txt")}}
	r := servicedRuntime(t, filepath.Join("installed", "local.resize-image-half"), rig, installedManifest(t, raw))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "resize-image-to-half"}})
	if view.Subtitle != "1 image selected · Enter to resize & compress" || len(view.Items) != 1 {
		t.Fatalf("view = %q with %d rows, want the one image counted", view.Subtitle, len(view.Items))
	}
	item := view.Items[0]
	if item.Title != "photo.png" || item.Subtitle != "4000 × 3000 → 2000 × 1500 · 2 KB" {
		t.Fatalf("row = %q / %q, want the size, the half size and the file size", item.Title, item.Subtitle)
	}
	if item.Primary == nil || item.Primary.Title != "Resize & Compress (50%)" {
		t.Fatalf("primary = %+v, want Resize & Compress (50%%)", item.Primary)
	}
}
