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

func stageManagerRuntime(t *testing.T, commands *fake.Commands) *Runtime {
	t.Helper()
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.stage-manager-stages")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	rig.deps.OS = "darwin"
	return servicedRuntime(t, filepath.Join("installed", "local.stage-manager-stages"), rig, installedManifest(t, raw))
}

// TestInstalledStageManagerListsShelfStagesAndSwitches runs the installed Stage Manager Stages extension against a
// scripted accessibility scan: it lists each shelf button once, skips lines that are not shelf buttons, and each row
// switches to its stage and closes the palette.
func TestInstalledStageManagerListsShelfStagesAndSwitches(t *testing.T) {
	requireUnixProcesses(t)
	commands := fake.NewCommands()
	commands.Script("osascript",
		fake.HelperScript{Stdout: []byte("WindowManager.1.1.1|Safari – Docs\nWindowManager.1.1.2|Notes\nnot a shelf line\nWindowManager.1.1.1|duplicate\n")},
		fake.HelperScript{Stdout: []byte("Safari|Safari – Docs\n")},
	)
	r := stageManagerRuntime(t, commands)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := r.Load(ctx, "stages")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var titles []string
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	if strings.Join(titles, ",") != "Safari – Docs,Notes" {
		t.Fatalf("stages = %v, want the two shelf buttons once each", titles)
	}
	first := items[0].Primary
	if first == nil || first.Title != "Switch to Safari – Docs" || first.Ref.Args["path"] != "WindowManager.1.1.1" || !first.DismissAfterRun {
		t.Fatalf("primary = %+v, want a switch to the first button that closes the palette", first)
	}
}

// TestInstalledStageManagerOffersADiagnosisWhenNoShelfIsExposed checks the empty case: with no shelf buttons the list
// has one row that opens the accessibility diagnosis, instead of an empty list with no way forward.
func TestInstalledStageManagerOffersADiagnosisWhenNoShelfIsExposed(t *testing.T) {
	requireUnixProcesses(t)
	commands := fake.NewCommands()
	commands.Script("osascript", fake.HelperScript{Stdout: []byte("")})
	r := stageManagerRuntime(t, commands)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := r.Load(ctx, "stages")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Inspect Stage Manager access" || items[0].Primary == nil {
		t.Fatalf("items = %+v, want the single diagnosis row", items)
	}
}
