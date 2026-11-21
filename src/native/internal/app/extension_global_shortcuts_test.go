package app

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

func TestGlobalShortcutTableDeclaresOnlyCommandsWithAShortcut(t *testing.T) {
	table := globalShortcutTableOf([]manager.CommandResult{{
		ExtensionID: "acme.notes",
		Commands: []api.Command{
			{ID: "capture", Title: "Capture", GlobalShortcut: "CommandOrControl+Alt+C", Mode: api.ModeNoView},
			{ID: "open", Title: "Open"},
		},
	}})
	if len(table.declared) != 1 || table.declared[0].ActionID != "extension:acme.notes:capture" ||
		table.declared[0].Accelerator != "CommandOrControl+Alt+C" || table.declared[0].Title != "Capture" {
		t.Fatalf("declared = %+v, want only the capture command under its default key", table.declared)
	}
	if _, ok := table.commands["extension:acme.notes:open"]; !ok {
		t.Fatalf("commands = %+v, want every listed command by its key", table.commands)
	}
}

func TestGlobalShortcutTableKeysCommandsByTheirActionID(t *testing.T) {
	table := globalShortcutTableOf([]manager.CommandResult{{
		ExtensionID: "nevermind.window-management",
		Commands: []api.Command{{
			ID: "layout.left-half", ActionID: "extension-action:window-management:layout.left-half", Title: "Left Half",
		}},
	}})
	entry, ok := table.commands["extension-action:window-management:layout.left-half"]
	if !ok || entry.extID != "nevermind.window-management" || entry.command.ID != "layout.left-half" {
		t.Fatalf("commands = %+v, want the layout under its 1.x action key", table.commands)
	}
	if len(table.declared) != 0 {
		t.Fatalf("declared = %+v, want none without a global shortcut", table.declared)
	}
}

func TestFailedExtensionContributesNoShortcuts(t *testing.T) {
	table := globalShortcutTableOf([]manager.CommandResult{{
		ExtensionID: "acme.broken",
		Commands:    []api.Command{{ID: "run", Title: "Run", GlobalShortcut: "Command+Alt+R"}},
		Failure:     &api.Failure{Code: api.CodeInternal, Message: "crashed"},
	}})
	if len(table.declared) != 0 || len(table.commands) != 0 {
		t.Fatalf("table = %+v, want nothing from a failed extension", table)
	}
}

func TestInPlaceRunIsOnlyForCommandsThatNeverOpenThePalette(t *testing.T) {
	cases := map[api.BackgroundMode]bool{
		api.ModeNoView: true, api.ModeBackground: true, api.ModeView: false, "": false,
	}
	for mode, want := range cases {
		if got := runsWithoutPalette(api.Command{Mode: mode}); got != want {
			t.Errorf("runsWithoutPalette(%q) = %v, want %v", mode, got, want)
		}
	}
}

func TestCommandShortcutKeyFollowsTheCommandsActionID(t *testing.T) {
	if got := commandShortcutKey("acme", api.Command{ID: "run"}); got != "extension:acme:run" {
		t.Fatalf("default key = %q", got)
	}
	if got := commandShortcutKey("acme", api.Command{ID: "run", ActionID: "extension-action:acme:run"}); got != "extension-action:acme:run" {
		t.Fatalf("action key = %q", got)
	}
}

func TestShortcutOutcomeReportsFailuresAndToasts(t *testing.T) {
	cases := []struct {
		name    string
		result  api.Result
		failure *api.Failure
		err     error
		want    string
		ok      bool
	}{
		{name: "failure", failure: &api.Failure{Message: "window is gone"}, want: "window is gone", ok: true},
		{name: "error", err: errors.New("no window"), want: "no window", ok: true},
		{name: "toast", result: api.Result{Toast: &api.Toast{Message: "Not on this display"}}, want: "Not on this display", ok: true},
		{name: "silent", want: "", ok: false},
	}
	for _, c := range cases {
		got, ok := shortcutOutcomeMessage(c.result, c.failure, c.err)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: shortcutOutcomeMessage = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}
