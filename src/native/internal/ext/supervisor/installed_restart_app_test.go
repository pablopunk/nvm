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

const restartPS = `  501 /Applications/Safari.app/Contents/MacOS/Safari
  502 /Applications/Safari.app/Contents/Frameworks/Helper.app/Contents/MacOS/Helper
  700 /System/Applications/Finder.app/Contents/MacOS/Finder
  701 /Users/pablo/Applications/Tool.app/Contents/MacOS/Tool
  777 /Applications/Nevermind.app/Contents/MacOS/Nevermind
  800 /usr/libexec/not-an-app
`

// TestInstalledRestartAppOffersRestartForceAndQuitPerRunningApp runs the installed Restart App extension against a
// scripted ps: apps from /Applications, /System/Applications and the user's Applications are listed by name, Nevermind
// is not, and each row's menu has Restart as the main action, then Force Restart and Quit as destructive ones, then
// the Finder and path actions.
func TestInstalledRestartAppOffersRestartForceAndQuitPerRunningApp(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.restart-app")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := installedManifest(t, raw)
	commands := fake.NewCommands()
	commands.Script("/bin/ps", fake.HelperScript{Stdout: []byte(restartPS)})
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	r := servicedRuntime(t, filepath.Join("installed", "local.restart-app"), rig, manifest)
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
	if strings.Join(titles, ",") != "Finder,Safari,Tool" {
		t.Fatalf("apps = %v, want Finder, Safari and Tool", titles)
	}
	safari := items[1]
	if len(safari.Accessories) != 1 || safari.Accessories[0].Text != "2 processes" {
		t.Fatalf("Safari accessories = %+v, want 2 processes", safari.Accessories)
	}
	var got []string
	for _, action := range safari.Actions {
		got = append(got, action.Title)
	}
	want := "Restart Safari,Force Restart Safari,Quit Safari,Reveal in Finder,Copy App Path"
	if strings.Join(got, ",") != want {
		t.Fatalf("actions = %v, want %s", got, want)
	}
	if safari.Primary == nil || safari.Primary.Title != "Restart Safari" || safari.Primary.Ref.Args["pids"] != "501,502" {
		t.Fatalf("primary = %+v, want Restart Safari for 501 and 502", safari.Primary)
	}
	for _, i := range []int{1, 2} {
		if safari.Actions[i].Style != "destructive" {
			t.Errorf("action %q style = %q, want destructive", safari.Actions[i].Title, safari.Actions[i].Style)
		}
	}
	if !safari.Primary.DismissAfterRun {
		t.Errorf("Restart must close the palette after it runs")
	}
}
