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

func installedRuntime(t *testing.T, id string, configure func(*servicesRig)) *Runtime {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "extensions", "installed", id, "extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	rig := newServicesRig(t)
	rig.deps.Home = t.TempDir()
	if configure != nil {
		configure(&rig)
	}
	return servicedRuntime(t, filepath.Join("installed", id), rig, installedManifest(t, raw))
}

func runInstalledResult(t *testing.T, ctx context.Context, r *Runtime, in api.Invocation) api.Result {
	t.Helper()
	response, err := r.Call(ctx, protocol.MethodRun, "", &in)
	if err != nil {
		t.Fatalf("run %q: %v", in.Ref.ID, err)
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil {
		t.Fatalf("run %q answered %s: %v", in.Ref.ID, response.Data, err)
	}
	return result
}

// TestInstalledCaffeinateStartsAndStopsKeepingTheMacAwake runs the installed Caffeinate extension: caffeinate starts the
// system tool in the background and says so, and decaffeinate stops it and says sleep is back, whether or not it ran.
func TestInstalledCaffeinateStartsAndStopsKeepingTheMacAwake(t *testing.T) {
	requireUnixProcesses(t)
	commands := fake.NewCommands()
	commands.Script("/bin/bash", fake.HelperScript{})
	commands.Script("/usr/bin/killall", fake.HelperScript{ExitCode: 1, Stderr: []byte("No matching processes belonging to you were found\n")})
	r := installedRuntime(t, "local.caffeinate", func(rig *servicesRig) { rig.deps.Commands = commands })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	on := runInstalledResult(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "caffeinate"}})
	if on.Toast == nil || !strings.Contains(on.Toast.Message, "stay awake") {
		t.Fatalf("caffeinate toast = %+v, want the awake confirmation", on.Toast)
	}
	off := runInstalledResult(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "decaffeinate"}})
	if off.Toast == nil || off.Toast.Message != "Sleep enabled. Caffeinate deactivated." {
		t.Fatalf("decaffeinate toast = %+v, want sleep enabled even when nothing was running", off.Toast)
	}
	calls := commands.Calls()
	if len(calls) != 2 || !strings.Contains(strings.Join(calls[0].Args, " "), "/usr/bin/caffeinate -di") || calls[1].Binary != "/usr/bin/killall" {
		t.Fatalf("calls = %+v, want caffeinate -di started in the background and killall to stop it", calls)
	}
}

// TestInstalledSyncthingBrowserOpensTheLocalWebUI checks the command hands the Syncthing page to the browser.
func TestInstalledSyncthingBrowserOpensTheLocalWebUI(t *testing.T) {
	requireUnixProcesses(t)
	r := installedRuntime(t, "local.syncthing-browser", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := runInstalledResult(t, ctx, r, api.Invocation{Ref: api.ActionRef{ID: "syncthing-browser"}})
	if result.Next == nil || result.Next.Kind != api.ActionOpenURL || result.Next.URL != "http://127.0.0.1:8384/" {
		t.Fatalf("next = %+v, want the Syncthing page opened", result.Next)
	}
}

const ghRepos = `[
 {"id":1,"name":"old","full_name":"pablopunk/old","html_url":"https://github.com/pablopunk/old","description":null,"private":false,"fork":false,"language":"Go","updated_at":"2020-01-01T00:00:00Z"},
 {"id":2,"name":"new","full_name":"pablopunk/new","html_url":"https://github.com/pablopunk/new","description":"newest","private":true,"fork":false,"language":null,"updated_at":"2021-06-01T00:00:00Z"},
 {"id":3,"name":"theirs","full_name":"someone/theirs","html_url":"https://github.com/someone/theirs","description":null,"private":false,"fork":true,"language":"C","updated_at":"2022-01-01T00:00:00Z"},
 {"id":4,"name":"forked","full_name":"pablopunk/forked","html_url":"https://github.com/pablopunk/forked","description":null,"private":false,"fork":true,"language":"Lua","updated_at":"2020-06-01T00:00:00Z"}]`

// TestInstalledGitHubReposListsOnlyOwnedReposNewestFirst runs the installed My GitHub Repos extension against a scripted
// gh: other owners' repositories are left out, the rest come newest first with their visibility, language and fork
// marks, and each row opens its repository. Without gh signed in, the failure says how to sign in.
func TestInstalledGitHubReposListsOnlyOwnedReposNewestFirst(t *testing.T) {
	requireUnixProcesses(t)
	commands := fake.NewCommands()
	commands.Script("gh",
		fake.HelperScript{Stdout: []byte(ghRepos)},
		fake.HelperScript{ExitCode: 4, Stderr: []byte("To get started with GitHub CLI, please run:  gh auth login\n")},
	)
	r := installedRuntime(t, "local.my-github-repos", func(rig *servicesRig) { rig.deps.Commands = commands })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := r.Load(ctx, "repos")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var names []string
	for _, item := range items {
		names = append(names, item.Title)
	}
	if strings.Join(names, ",") != "new,forked,old" {
		t.Fatalf("repos = %v, want the owned ones newest first", names)
	}
	newest := items[0]
	if len(newest.Accessories) != 1 || newest.Accessories[0].Text != "Private" || newest.Primary == nil || newest.Primary.URL != "https://github.com/pablopunk/new" {
		t.Fatalf("newest = %+v, want Private and a link to the repository", newest)
	}
	if forked := items[1]; len(forked.Accessories) != 3 || forked.Accessories[2].Text != "Fork" {
		t.Fatalf("forked accessories = %+v, want Public, the language and Fork", forked.Accessories)
	}
	if _, err := r.Load(ctx, "repos"); err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("signed-out load = %v, want the sign-in instruction", err)
	}
}
