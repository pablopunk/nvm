package app

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// TestSystemSettingsRowHasItsStableID maps the System Settings row to the 1.x command key's ID.
func TestSystemSettingsRowHasItsStableID(t *testing.T) {
	if got := builtInRowActionID(systemcommands.ID, search.Item{ID: "open-system-settings"}); got != shortcuts.CommandSystemSettings {
		t.Fatalf("stable ID of System Settings = %q", got)
	}
}

// TestSystemCommandRowKeepsItsShortcutUnderAStableID binds Lock Screen by its stable ID and finds the row again: the
// shortcut shows on the row and its menu offers Remove keyboard shortcut, whatever the row's title.
func TestSystemCommandRowKeepsItsShortcutUnderAStableID(t *testing.T) {
	if got := builtInRowActionID(systemcommands.ID, search.Item{ID: "lock-screen", Title: "Lock Screen"}); got != shortcuts.CommandLockScreen {
		t.Fatalf("stable ID of Lock Screen = %q", got)
	}
	p := goldenPalette(t)
	mustSetShortcut(t, p.shortcutsManager(), shortcuts.CommandLockScreen, "Command+Alt+L")
	goldenQuery(p, "lock screen")
	_, row := rootRowTitled(t, p, "Lock Screen")
	if want := []string{"Lock Screen", setShortcutTitle, removeShortcutTitle, setAliasTitle}; !slices.Equal(searchActionTitles(row.Actions), want) {
		t.Fatalf("actions = %v, want %v", searchActionTitles(row.Actions), want)
	}
	if row.Accessory != shortcutLabelOf("Command+Alt+L") {
		t.Fatalf("accessory = %q", row.Accessory)
	}
}
