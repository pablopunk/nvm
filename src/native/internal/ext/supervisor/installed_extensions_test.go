package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// The fixtures under testdata/extensions/installed are copies of the extensions a real 1.x user converted to 2.x. Their
// platform commands (Finder, osascript, ffmpeg) cannot succeed on the Linux rig, but every one of them must still load
// against the current extension API, list valid commands, and answer each command with a valid result or a plain
// failure: never a crash, an invalid view, or a refusal because its manifest lacks a capability its code uses.
func TestInstalledExtensionsLoadAndAnswerEveryCommand(t *testing.T) {
	requireUnixProcesses(t)
	entries, err := os.ReadDir(filepath.Join("..", "testdata", "extensions", "installed"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		t.Run(name, func(t *testing.T) { checkInstalledExtension(t, name) })
	}
}

func checkInstalledExtension(t *testing.T, id string) {
	dir := filepath.Join("..", "testdata", "extensions", "installed", id)
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest api.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	r := servicedRuntime(t, filepath.Join("installed", id), rig, manifest)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	response, err := r.Call(ctx, protocol.MethodCommands, "", nil)
	if err != nil {
		if r.Provides(protocol.MethodCommands) {
			t.Fatalf("commands: %v", err)
		}
		return
	}
	var commands []api.Command
	if err := json.Unmarshal(response.Data, &commands); err != nil {
		t.Fatalf("commands decode: %v", err)
	}
	for _, command := range commands {
		if err := command.Validate(); err != nil {
			t.Errorf("command %q is invalid: %v", command.ID, err)
		}
		checkCommandAnswer(t, ctx, r, command)
	}
}

func checkCommandAnswer(t *testing.T, ctx context.Context, r *Runtime, command api.Command) {
	t.Helper()
	response, err := r.Call(ctx, protocol.MethodRun, "", &api.Invocation{Ref: api.ActionRef{ID: command.ID}})
	if err != nil {
		var callErr *CallError
		if !errors.As(err, &callErr) {
			t.Errorf("command %q: %v", command.ID, err)
			return
		}
		if strings.Contains(callErr.Failure.Message, "did not declare") {
			t.Errorf("command %q needs a capability its manifest does not declare: %s", command.ID, callErr.Failure.Message)
		}
		return
	}
	if response.Failure != nil && strings.Contains(response.Failure.Message, "did not declare") {
		t.Errorf("command %q needs a capability its manifest does not declare: %s", command.ID, response.Failure.Message)
		return
	}
	var result api.Result
	if len(response.Data) == 0 || json.Unmarshal(response.Data, &result) != nil || result.View == nil {
		return
	}
	if err := result.View.Validate(); err != nil {
		t.Errorf("command %q answered an invalid view: %v", command.ID, err)
	}
}
