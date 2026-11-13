package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
	"github.com/pablopunk/nvm/native/internal/store"
)

const macOnlyCommandSource = `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "today", Title: "Show Today", GlobalShortcut: "CommandOrControl+Alt+T"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"Calendar\" to activate", api.ShellOptions{})
	return api.Result{}, err
}
`

// macOnlyInstance is an app on goos with a synced, enabled, macOS-only extension installed.
func macOnlyInstance(t *testing.T, goos string) (*App, *manager.Manager) {
	t.Helper()
	dir := t.TempDir()
	ports := real.NewPorts()
	state, err := store.Open(ports.FS, ports.Clock, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, err := manager.New(manager.Ports{Files: ports.FS, Clock: ports.Clock, Worker: real.NewWorker(), Processes: ports.Processes, OS: goos},
		filepath.Join(dir, "extensions"), enablement.Open(state), manager.Options{Executable: exe, DataRoot: filepath.Join(dir, "extension-data")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	files := map[string][]byte{
		"extension.json": []byte(`{"apiVersion":"1.0","id":"mac.calendar","title":"Calendar","capabilities":["system"]}`),
		"main.go":        []byte(macOnlyCommandSource),
	}
	if err := m.Install("mac.calendar", files, true); err != nil {
		t.Fatal(err)
	}
	app := &App{Palette: palette.New(palette.Options{}), ports: fake.NewSet().Ports()}
	app.exts.mgr.Store(m)
	app.exts.settled.settle()
	return app, m
}

// TestAMacOnlyExtensionIsListedPlainlyAndBindsNothingOnLinux: the Extensions view says "macOS only" in the state, not as
// a problem; no shortcut is declared for it; and running its command is refused with the reason, quietly.
func TestAMacOnlyExtensionIsListedPlainlyAndBindsNothingOnLinux(t *testing.T) {
	app, m := macOnlyInstance(t, api.OSLinux)
	var listing manager.Listing
	for _, l := range m.List() {
		if l.ID == "mac.calendar" {
			listing = l
		}
	}
	row := extensionListRow(app, &fakeExtensions{listings: []manager.Listing{listing}}, listing)
	if row.Accessory != "macOS only" || row.Tone != "gray" || strings.Contains(row.Subtitle, "·") {
		t.Fatalf("row accessory %q tone %q subtitle %q; want a plain macOS-only note", row.Accessory, row.Tone, row.Subtitle)
	}
	if app.extensionMayRun("mac.calendar") {
		t.Fatal("the extension may run on Linux")
	}
	m.CommandsStarted(context.Background())
	if table := globalShortcutTableOf(m.CachedCommands()); len(table.declared) != 0 || len(table.commands) != 0 {
		t.Fatalf("shortcut table = %+v; want nothing declared or runnable", table)
	}
	if key := commandShortcutKey("mac.calendar", api.Command{ID: "today"}); app.actionRunnable(key) {
		t.Fatalf("the key %q of a macOS-only command is bound on Linux; the Hyprland module would get it", key)
	}
	ctx := context.Background()
	response := app.runActionCommand(ctx, "extension:mac.calendar:today")
	if response.Outcome != actioncli.Rejected || !strings.Contains(response.Message, "macOS only") || !strings.Contains(response.Message, "nothing ran") {
		t.Fatalf("response %+v; want a quiet refusal that says macOS only", response)
	}
}

// TestAMacOnlyExtensionRunsAsBeforeOnDarwin: the same install lists its command and its shortcut.
func TestAMacOnlyExtensionRunsAsBeforeOnDarwin(t *testing.T) {
	app, m := macOnlyInstance(t, api.OSDarwin)
	m.CommandsStarted(context.Background())
	table := globalShortcutTableOf(m.CachedCommands())
	if len(table.declared) != 1 || !app.extensionMayRun("mac.calendar") {
		t.Fatalf("shortcut table %+v, may run %v; want the shortcut bound on macOS", table.declared, app.extensionMayRun("mac.calendar"))
	}
}
