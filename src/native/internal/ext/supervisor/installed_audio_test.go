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

const speakersBefore = `[{"uid":"speakers","name":"MacBook Pro Speakers","isDefault":true,"channels":2},{"uid":"razer","name":"Razer Nommo","isDefault":false,"channels":2}]`
const speakersAfter = `[{"uid":"speakers","name":"MacBook Pro Speakers","isDefault":false,"channels":2},{"uid":"razer","name":"Razer Nommo","isDefault":true,"channels":2}]`

// TestInstalledAudioDevicesListsOutputsAndSwitchesTheDefault runs the installed Audio Devices extension against a
// scripted CoreAudio helper: the list marks the current default and gives the others their channel count, and choosing
// another device answers with the list showing it as the new default.
func TestInstalledAudioDevicesListsOutputsAndSwitchesTheDefault(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.audio-devices")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := installedManifest(t, raw)
	commands := fake.NewCommands()
	ok := fake.HelperScript{}
	out := func(text string) fake.HelperScript { return fake.HelperScript{Stdout: []byte(text)} }
	commands.Script("/bin/bash",
		ok, out(speakersBefore), // load: tool present, list
		ok, ok, ok, out(speakersAfter), // switch: tool present, set, tool present, list
	)
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	r := servicedRuntime(t, filepath.Join("installed", "local.audio-devices"), rig, manifest)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := r.Load(ctx, "devices:output")
	if err != nil {
		t.Fatalf("load outputs: %v", err)
	}
	if len(items) != 2 || items[0].Title != "MacBook Pro Speakers" || items[0].Subtitle != "Current Default" || items[1].Subtitle != "2 channels" {
		t.Fatalf("items = %+v, want the default marked and the other with its channel count", items)
	}
	choose := items[1].Primary
	if choose == nil || choose.Title != "Set as Default" || choose.Ref == nil {
		t.Fatalf("primary action = %+v, want Set as Default", choose)
	}

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: *choose.Ref})
	if len(view.Items) != 2 || view.Items[1].Subtitle != "Current Default" || strings.Contains(view.Items[0].Subtitle, "Current") {
		t.Fatalf("after switching, items = %+v, want Razer Nommo as the default", view.Items)
	}
}
