package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const psOutput = `    1 /sbin/launchd
  501 /Applications/Safari.app/Contents/MacOS/Safari
  502 /Applications/Safari.app/Contents/Frameworks/Helper.app/Contents/MacOS/Helper
  610 /Applications/Visual Studio Code.app/Contents/MacOS/Electron
  777 /Applications/Nevermind.app/Contents/MacOS/Nevermind
  900 /System/Applications/Finder.app/Contents/MacOS/Finder
`

// TestInstalledQuitAppListsRunningAppsFromApplications runs the installed Quit App extension against a scripted ps: the
// list has one row per app bundle in /Applications, sorted by name, with its process id, and leaves out system apps and
// Nevermind itself. Each row's main action is a destructive kill of that app's processes.
func TestInstalledQuitAppListsRunningAppsFromApplications(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.quit-running-app")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := installedManifest(t, raw)
	commands := fake.NewCommands()
	commands.Script("/bin/ps", fake.HelperScript{Stdout: []byte(psOutput)})
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	r := servicedRuntime(t, filepath.Join("installed", "local.quit-running-app"), rig, manifest)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := r.Load(ctx, "running-apps")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var titles []string
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	if strings.Join(titles, ",") != "Safari,Visual Studio Code" {
		t.Fatalf("apps = %v, want Safari and Visual Studio Code only", titles)
	}
	safari := items[0]
	if len(safari.Accessories) != 1 || safari.Accessories[0].Text != "2 processes" {
		t.Fatalf("Safari accessories = %+v, want its two processes counted", safari.Accessories)
	}
	if safari.Primary == nil || safari.Primary.Title != "Kill Safari" || safari.Primary.Style != "destructive" || safari.Primary.Ref.Args["pids"] != "501,502" {
		t.Fatalf("Safari primary = %+v, want a destructive kill of 501 and 502", safari.Primary)
	}
	if code := items[1]; code.Accessories[0].Text != "PID 610" {
		t.Fatalf("Code accessories = %+v, want PID 610", code.Accessories)
	}
}
