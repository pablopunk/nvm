package manager

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// macOnlySource lists a command with a global shortcut, a root row and a search row, and runs AppleScript: an
// extension of the kind a synced Mac calendar is.
const macOnlySource = `package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "today", Title: "Show Today", GlobalShortcut: "CommandOrControl+Alt+T"}}, nil
}

func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "root", Title: "Calendar root"}}, nil
}

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	return []api.Item{{ID: "event", Title: "Calendar event"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	_, err := ctx.Desktop().Shell().AppleScript("tell application \"Calendar\" to activate", api.ShellOptions{})
	return api.Result{}, err
}
`

func macOnlyContents() map[string][]byte {
	return map[string][]byte{
		bundle.ManifestFile: []byte(`{"apiVersion":"1.0","id":"mac.calendar","title":"Calendar","capabilities":["system"]}`),
		bundle.SourceFile:   []byte(macOnlySource),
	}
}

func commandIDs(results []CommandResult) []string {
	var ids []string
	for _, result := range results {
		for _, command := range result.Commands {
			ids = append(ids, result.ExtensionID+":"+command.ID)
		}
	}
	return ids
}

// TestAMacOnlyExtensionDoesNothingOnLinux: it stays enabled, but it does not start and contributes no commands, rows,
// search results or shortcuts, and running it is refused with a plain message, not an error.
func TestAMacOnlyExtensionDoesNothingOnLinux(t *testing.T) {
	h := newHarnessOnOS(t, api.OSLinux)
	if err := h.m.Install("mac.calendar", macOnlyContents(), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	listing := h.listing("mac.calendar")
	if !listing.Enabled || !listing.UnsupportedHere || listing.OnlyOn != "macOS only" || listing.Runtime.State != supervisor.StateIdle || len(listing.Problems) != 0 {
		t.Fatalf("listing = %+v; want enabled, macOS only, idle, no problem", listing)
	}
	ctx := context.Background()
	if got := commandIDs(h.m.CommandsStarted(ctx)); len(got) != 0 {
		t.Fatalf("commands = %v; want none", got)
	}
	if got := commandIDs(h.m.CachedCommands()); len(got) != 0 {
		t.Fatalf("cached commands (the source of shortcuts) = %v; want none", got)
	}
	if got := h.m.Roots(ctx); len(got) != 0 {
		t.Fatalf("roots = %+v; want none", got)
	}
	if got := h.search("c"); len(got) != 0 {
		t.Fatalf("search = %+v; want none", got)
	}
	_, failure, err := h.m.Run(ctx, "mac.calendar", api.Invocation{Ref: api.ActionRef{ID: "today"}})
	if err != nil || failure == nil || failure.Code != api.CodeUnsupported || failure.Message != "Calendar only works on macOS" {
		t.Fatalf("run = failure %+v err %v; want the quiet macOS-only refusal", failure, err)
	}
	if state := h.listing("mac.calendar").Runtime.State; state != supervisor.StateIdle {
		t.Fatalf("state = %s; the extension started", state)
	}
	if !h.listing("mac.calendar").Enabled {
		t.Fatal("the enabled choice was lost; it must stay on for the Mac")
	}
	if err := h.m.Enable("mac.calendar"); err != nil || h.listing("mac.calendar").Runtime.State != supervisor.StateIdle {
		t.Fatalf("enabling again started it: err %v", err)
	}
}

// TestAMacOnlyExtensionBehavesAsBeforeOnDarwin: it starts and lists its commands, rows and shortcuts.
func TestAMacOnlyExtensionBehavesAsBeforeOnDarwin(t *testing.T) {
	h := newHarnessOnOS(t, api.OSDarwin)
	if err := h.m.Install("mac.calendar", macOnlyContents(), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	listing := h.listing("mac.calendar")
	if !listing.Enabled || listing.UnsupportedHere || listing.OnlyOn != "macOS only" {
		t.Fatalf("listing = %+v; want enabled and supported, labelled macOS only", listing)
	}
	ctx := context.Background()
	got := h.m.CommandsStarted(ctx)
	if ids := commandIDs(got); len(ids) != 1 || ids[0] != "mac.calendar:today" || got[0].Commands[0].GlobalShortcut == "" {
		t.Fatalf("commands = %v; want the command with its shortcut", ids)
	}
	if roots := h.m.Roots(ctx); len(roots) != 1 || len(roots[0].Items) != 1 {
		t.Fatalf("roots = %+v; want the root row", roots)
	}
}

// TestAnExtensionThatWorksEverywhereRunsOnLinux: ordinary extensions are untouched.
func TestAnExtensionThatWorksEverywhereRunsOnLinux(t *testing.T) {
	h := newHarnessOnOS(t, api.OSLinux)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.good")
	if listing := h.listing("acme.good"); listing.UnsupportedHere || listing.OnlyOn != "" {
		t.Fatalf("listing = %+v; want no platform limit", listing)
	}
}
