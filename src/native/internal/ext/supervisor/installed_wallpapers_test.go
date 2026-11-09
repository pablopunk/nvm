package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestInstalledWallpapersGridListsTheImagesOfTheWallpaperFolder runs the installed Wallpapers extension against a real
// ~/.config/wallpapers folder: the grid lists every image in it, one folder level down too and sorted by name, leaves
// documents out, and each picture offers Set Wallpaper & Match Colors first, then the preview, restore and file actions.
func TestInstalledWallpapersGridListsTheImagesOfTheWallpaperFolder(t *testing.T) {
	requireUnixProcesses(t)
	home := t.TempDir()
	folder := filepath.Join(home, ".config", "wallpapers")
	for _, name := range []string{"b-dunes.jpg", "a-forest.png", "sub/c-lake.png", "notes.txt"} {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.wallpapers")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	commands := fake.NewCommands()
	commands.Script("/bin/sh", fake.HelperScript{Stdout: []byte(home)})
	rig := newServicesRig(t)
	rig.deps.Home = home
	rig.deps.Commands = commands
	rig.deps.OS = "darwin"
	r := servicedRuntime(t, filepath.Join("installed", "local.wallpapers"), rig, installedManifest(t, raw))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "wallpapers.grid"}})
	var names []string
	for _, item := range view.Items {
		names = append(names, item.Title)
	}
	if strings.Join(names, ",") != "a-forest.png,b-dunes.jpg,c-lake.png" {
		t.Fatalf("wallpapers = %v, want the three images sorted by name and no document", names)
	}
	first := view.Items[0]
	if first.Primary == nil || first.Primary.Title != "Set Wallpaper & Match Colors" {
		t.Fatalf("primary = %+v, want Set Wallpaper & Match Colors", first.Primary)
	}
	var actions []string
	for _, action := range first.Actions {
		actions = append(actions, action.Title)
	}
	want := "Set Wallpaper Only,Preview Matching Colors,Restore Previous Colors,Refresh Wallpaper Icon Appearance,Quick Look,Reveal in Finder,Open Image,Copy Path"
	if strings.Join(actions, ",") != want {
		t.Fatalf("actions = %v, want %s", actions, want)
	}
	if view.Subtitle != "~/.config/wallpapers" || view.Refresh == nil {
		t.Fatalf("subtitle = %q, refresh = %+v, want the folder named and a refresh", view.Subtitle, view.Refresh)
	}
}
