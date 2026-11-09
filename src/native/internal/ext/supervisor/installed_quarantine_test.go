package supervisor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func hasQuarantine() fake.HelperScript { return fake.HelperScript{} }
func noQuarantine() fake.HelperScript {
	return fake.HelperScript{ExitCode: 1, Stderr: []byte("xattr: /x: No such xattr: com.apple.quarantine\n")}
}

// TestInstalledRemoveQuarantineListsQuarantinedAppsAndRemovesOne runs the installed Remove App Quarantine extension
// against a scripted find and xattr: the list shows only quarantined apps, sorted, each asking to confirm before it
// removes, and removing one answers with the list without it.
func TestInstalledRemoveQuarantineListsQuarantinedAppsAndRemovesOne(t *testing.T) {
	requireUnixProcesses(t)
	dir := filepath.Join("..", "testdata", "extensions", "installed", "local.remove-app-quarantine")
	raw, err := os.ReadFile(filepath.Join(dir, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := installedManifest(t, raw)
	commands := fake.NewCommands()
	find := fake.HelperScript{Stdout: []byte("/Applications/Beta.app\n/Applications/Alpha.app\n/Applications/Clean.app\n")}
	commands.Script("/bin/bash", find, find)
	commands.Script("/usr/bin/xattr",
		hasQuarantine(), hasQuarantine(), noQuarantine(), // listing: Beta, Alpha, Clean
		hasQuarantine(), hasQuarantine(), noQuarantine(), // removing Alpha: check, delete, verify
		hasQuarantine(), noQuarantine(), noQuarantine(), // listing again: Beta, Alpha, Clean
	)
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	rig.deps.Commands = commands
	r := servicedRuntime(t, filepath.Join("installed", "local.remove-app-quarantine"), rig, manifest)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	view := runInstalledView(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "remove-app-quarantine"}})
	assertQuarantineList(t, view, []string{"Alpha.app", "Beta.app"})
	primary := view.Items[0].Primary
	if primary == nil || primary.Confirm == nil || !primary.Confirm.Destructive {
		t.Fatalf("remove action = %+v, want one that asks for a destructive confirmation", primary)
	}

	after := runInstalledView(t, ctx, r, api.Invocation{Ref: *primary.Ref})
	assertQuarantineList(t, after, []string{"Beta.app"})
}

func assertQuarantineList(t *testing.T, view api.View, want []string) {
	t.Helper()
	var got []string
	for _, item := range view.Items {
		got = append(got, item.Title)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("apps = %v, want %v", got, want)
	}
}

// runInstalledView runs one invocation and returns the view the extension answers with.
func runInstalledView(t *testing.T, ctx context.Context, r *Runtime, in api.Invocation) api.View {
	t.Helper()
	response, err := r.Call(ctx, protocol.MethodRun, "", &in)
	if err != nil {
		t.Fatalf("run %q: %v", in.Ref.ID, err)
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil || result.View == nil {
		t.Fatalf("run %q answered %s (%v), want a view", in.Ref.ID, response.Data, err)
	}
	return *result.View
}
