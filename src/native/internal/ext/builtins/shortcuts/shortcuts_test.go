package shortcuts

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeShortcuts is the host's shortcut table: the bound records the built-in lists.
type fakeShortcuts struct {
	api.Shortcuts
	records []api.ShortcutRecord
	palette api.ShortcutRecord
}

func (f fakeShortcuts) List() []api.ShortcutRecord { return f.records }

func (f fakeShortcuts) Palette() api.ShortcutRecord { return f.palette }

// fakeContext is the context the built-in runs with, over one fake table.
type fakeContext struct {
	api.Context
	shortcuts fakeShortcuts
}

func (c fakeContext) Shortcuts() api.Shortcuts { return c.shortcuts }

func fixtureShortcuts() fakeShortcuts {
	return fakeShortcuts{records: []api.ShortcutRecord{
		{ActionID: "command:dictate", Title: "Dictate", Accelerator: "Command+Control+Alt+Shift+V", Label: "✦V", Scope: "global", Source: "user"},
		{ActionID: "command:clipboard", Title: "Clipboard History", Accelerator: "Command+Alt+V", Label: "⌘⌥V", Scope: "global", Source: "extension"},
	}}
}

func openPage(t *testing.T, table fakeShortcuts) api.View {
	t.Helper()
	result, err := New().RunAction(fakeContext{shortcuts: table}, api.Invocation{Ref: api.ActionRef{ID: openAction}})
	if err != nil || result.View == nil {
		t.Fatalf("open: %v, %+v", err, result)
	}
	return *result.View
}

func TestPageListsTheBoundShortcutsWithTheirLabels(t *testing.T) {
	view := openPage(t, fixtureShortcuts())
	if view.Title != "Keyboard Shortcuts" || view.Placeholder != "Search Keyboard Shortcuts" || view.Hint != "↵ Change" {
		t.Fatalf("page = %q, %q, %q", view.Title, view.Placeholder, view.Hint)
	}
	if len(view.Items) != 3 {
		t.Fatalf("rows = %d, want the palette row and 2", len(view.Items))
	}
	if got := view.Items[1].Accessories[0].Text; got != "✦V" {
		t.Fatalf("hyper label = %q, want ✦V", got)
	}
	if got := view.Items[2].Accessories[0].Text; got != "⌘⌥V" {
		t.Fatalf("clipboard label = %q", got)
	}
}

func TestEnterChangesAShortcutAndOnlyTheUsersChoicesCanBeRemoved(t *testing.T) {
	view := openPage(t, fixtureShortcuts())
	user := view.Items[1]
	if user.Primary == nil || user.Primary.Kind != api.ActionShortcut || user.Primary.Op != api.ShortcutOpRecord || user.Primary.Target != "command:dictate" {
		t.Fatalf("primary = %+v", user.Primary)
	}
	if len(user.Actions) != 3 || user.Actions[1].Op != api.ShortcutOpRemove || user.Actions[1].Title != "Remove shortcut" ||
		user.Actions[2].Op != api.ShortcutOpCopyCommand || user.Actions[2].Target != "command:dictate" {
		t.Fatalf("user actions = %+v", user.Actions)
	}
	declared := view.Items[2]
	if len(declared.Actions) != 2 || declared.Actions[0].Op != api.ShortcutOpRecord || declared.Actions[1].Op != api.ShortcutOpCopyCommand {
		t.Fatalf("declared actions = %+v", declared.Actions)
	}
}

func TestAnEmptyTableStillListsTheNevermindShortcut(t *testing.T) {
	view := openPage(t, fakeShortcuts{palette: api.ShortcutRecord{ActionID: "palette"}})
	if len(view.Items) != 1 || view.Items[0].Title != "Open Nevermind" || view.Items[0].Accessories[0].Text != "Not set" {
		t.Fatalf("view = %+v", view.Items)
	}
	if view.EmptyView == nil || view.EmptyView.Title != "No shortcuts found." {
		t.Fatalf("empty state = %+v", view.EmptyView)
	}
}

// TestTheNevermindShortcutIsFirstAndCanBeChangedButNotRemoved: without it the palette has no way to open.
func TestTheNevermindShortcutIsFirstAndCanBeChangedButNotRemoved(t *testing.T) {
	table := fixtureShortcuts()
	table.palette = api.ShortcutRecord{ActionID: "palette", Accelerator: "Command+Space", Label: "⌘Space", Source: "user"}
	row := openPage(t, table).Items[0]
	if row.Title != "Open Nevermind" || row.Accessories[0].Text != "⌘Space" || row.Primary == nil || row.Primary.Op != api.ShortcutOpRecord || row.Primary.Target != "palette" {
		t.Fatalf("palette row = %+v", row)
	}
	for _, a := range row.Actions {
		if a.Op == api.ShortcutOpRemove {
			t.Fatalf("the Nevermind shortcut offers %q", a.Title)
		}
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	_, err := New().RunAction(fakeContext{shortcuts: fixtureShortcuts()}, api.Invocation{Ref: api.ActionRef{ID: "bind"}})
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// TestRowsShowTheRecordLabelOnlyAndNeverTheStoredAccelerator checks that no row text spells the stored form.
func TestRowsShowTheRecordLabelOnlyAndNeverTheStoredAccelerator(t *testing.T) {
	table := fakeShortcuts{records: []api.ShortcutRecord{
		{ActionID: "command:mute", Title: "Mute", Accelerator: "Command+Control+Alt+M", Label: "⌘⌃⌥M", Scope: "global", Source: "user"},
		{ActionID: "command:dictate", Title: "Dictate", Accelerator: "Command+Control+Alt+Shift+V", Label: "✦V", Scope: "global", Source: "user"},
	}}
	for i, want := range []string{"⌘⌃⌥M", "✦V"} {
		row := openPage(t, table).Items[i+1]
		if row.Accessories[0].Text != want {
			t.Errorf("row %d accessory = %q, want %q", i, row.Accessories[0].Text, want)
		}
		for _, text := range []string{row.Title, row.Subtitle, row.Accessories[0].Text} {
			if strings.Contains(text, "Command+") {
				t.Errorf("row %d shows the stored accelerator: %q", i, text)
			}
		}
	}
}
