package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
	"github.com/pablopunk/nvm/native/internal/store"
)

const cliFixture = "acme.cli"

// cliInstance is one run of the app over a folder of user extensions: a manager over real folders and children, the
// way a command line start builds it, and the notifications the extension's command produced.
type cliInstance struct {
	app      *App
	manager  *manager.Manager
	notifier *fake.Notifier
	stop     func()
}

// startCLIInstance opens the extension folder under dir as a fresh start does. Nothing asks for commands: no palette
// has opened and no query has run.
func startCLIInstance(t *testing.T, dir string) cliInstance {
	t.Helper()
	ports := real.NewPorts()
	state, err := store.Open(ports.FS, ports.Clock, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, err := manager.New(manager.Ports{Files: ports.FS, Clock: ports.Clock, Worker: real.NewWorker(), Processes: ports.Processes},
		filepath.Join(dir, "extensions"), enablement.Open(state), manager.Options{Executable: exe, DataRoot: filepath.Join(dir, "extension-data")})
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}
	var once sync.Once
	stop := func() { once.Do(func() { m.Close(); _ = state.Close() }) }
	t.Cleanup(stop)
	set := fake.NewSet()
	app := &App{Palette: palette.New(palette.Options{}), ports: set.Ports()}
	app.exts.mgr.Store(m)
	app.exts.settled.settle()
	return cliInstance{app: app, manager: m, notifier: set.Notifier, stop: stop}
}

func (c cliInstance) runCLICommand(t *testing.T, actionID string) actioncli.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return c.app.runActionCommand(ctx, actionID)
}

func (c cliInstance) awaitPing(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if shown := c.notifier.Shown(); len(shown) == 1 && shown[0].Body == "pinged ping" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the command never ran; notifications %v", c.notifier.Shown())
}

func (c cliInstance) state(t *testing.T) supervisor.State {
	t.Helper()
	for _, listing := range c.manager.List() {
		if listing.ID == cliFixture {
			return listing.Runtime.State
		}
	}
	t.Fatalf("%s is not installed", cliFixture)
	return ""
}

// installCLIFixture installs the folder extension in a first run, lets it list its commands, and ends that run, so the
// next start finds the extension in its folder with its commands saved.
func installCLIFixture(t *testing.T, dir string) {
	t.Helper()
	first := startCLIInstance(t, dir)
	if err := first.manager.Install(cliFixture, fixtureFiles(t, cliFixture), true); err != nil {
		t.Fatalf("install: %v", err)
	}
	first.manager.CommandsStarted(context.Background())
	first.stop()
}

// A folder extension that only lists commands waits dormant on a start whose commands were saved. The command line must
// still run its command, with nothing having asked for the commands first.
func TestRunStartsADormantFolderExtensionsCommandOnAFreshStart(t *testing.T) {
	dir := t.TempDir()
	installCLIFixture(t, dir)

	second := startCLIInstance(t, dir)
	if got := second.state(t); got != supervisor.StateDormant {
		t.Fatalf("state %q, want dormant: the saved commands were not used", got)
	}
	if r := second.runCLICommand(t, "extension:"+cliFixture+":ping"); r.Outcome != actioncli.Started {
		t.Fatalf("run: %+v", r)
	}
	second.awaitPing(t)
}

// With no saved commands the extension is started by the run itself, and its commands are learned only once its child is
// up: the lookup must wait for that start instead of rejecting.
func TestRunStartsAFolderExtensionThatHasNoSavedCommands(t *testing.T) {
	dir := t.TempDir()
	installCLIFixture(t, dir)
	if err := os.Remove(filepath.Join(dir, "extension-data", ".command-snapshots.json")); err != nil {
		t.Fatal(err)
	}

	second := startCLIInstance(t, dir)
	if got := second.state(t); got == supervisor.StateDormant {
		t.Fatalf("state %q: the extension should have no saved commands to wait on", got)
	}
	if r := second.runCLICommand(t, "extension:"+cliFixture+":ping"); r.Outcome != actioncli.Started {
		t.Fatalf("run: %+v", r)
	}
	second.awaitPing(t)
}

// An extension that is not installed is still rejected, however long the lookup waited for the others.
func TestRunRejectsAnExtensionCommandNobodyListed(t *testing.T) {
	dir := t.TempDir()
	installCLIFixture(t, dir)
	second := startCLIInstance(t, dir)
	for _, id := range []string{"extension:" + cliFixture + ":missing", "extension:acme.absent:ping"} {
		if r := second.runCLICommand(t, id); r.Outcome != actioncli.Rejected {
			t.Fatalf("%s: %+v", id, r)
		}
	}
}

// A command of an installed extension that is turned off on this machine is refused with what to do, not as unknown:
// enablement syncs between machines, so the command can exist and still be off here.
func TestRunSaysAnInstalledExtensionIsTurnedOff(t *testing.T) {
	dir := t.TempDir()
	installCLIFixture(t, dir)
	second := startCLIInstance(t, dir)
	if err := second.manager.Disable(cliFixture); err != nil {
		t.Fatal(err)
	}
	r := second.runCLICommand(t, "extension:"+cliFixture+":ping")
	if r.Outcome != actioncli.Rejected || !strings.Contains(r.Message, "turned off") {
		t.Fatalf("response %+v, want a refusal that says the extension is off", r)
	}
}
