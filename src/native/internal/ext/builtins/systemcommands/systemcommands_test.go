package systemcommands

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeContext serves the System service with a fixed set of capabilities and the settings label; every other service
// is absent.
type fakeContext struct {
	api.Context
	supported map[string]bool
}

func (c fakeContext) System() api.System { return fakeSystem{supported: c.supported} }

type fakeSystem struct {
	api.System
	supported map[string]bool
}

func (s fakeSystem) HasCapability(name string) bool { return s.supported[name] }

func (s fakeSystem) Labels() api.SystemLabels {
	return api.SystemLabels{SettingsCommand: "Open System Settings"}
}

func allSupported() map[string]bool {
	names := []string{"system.lock", "system.sleep", "system.restart", "system.shutDown", "system.logOut", "system.emptyTrash", "system.settings"}
	out := map[string]bool{}
	for _, name := range names {
		out[name] = true
	}
	return out
}

func searchTitles(t *testing.T, supported map[string]bool, query string) []api.Item {
	t.Helper()
	items, err := Extension{}.SearchItems(fakeContext{supported: supported}, query)
	if err != nil {
		t.Fatalf("SearchItems(%q): %v", query, err)
	}
	return items
}

func TestEmptyQueryListsNoCommands(t *testing.T) {
	if items := searchTitles(t, allSupported(), ""); len(items) != 0 {
		t.Fatalf("items = %+v, want none for the empty root", items)
	}
}

func TestTitleMatchRunsItsOpAndKeepsTheTone(t *testing.T) {
	items := searchTitles(t, allSupported(), "restart")
	if len(items) != 1 || items[0].Title != "Restart Computer" {
		t.Fatalf("items = %+v, want the Restart Computer command", items)
	}
	item := items[0]
	if item.Section != "Commands" || item.LearnKey != "cmd:Restart Computer" || item.Subtitle != "Restart this computer" {
		t.Errorf("row = section %q learn %q subtitle %q", item.Section, item.LearnKey, item.Subtitle)
	}
	if item.Appearance == nil || item.Appearance.Foreground != api.ColorOrange {
		t.Errorf("appearance = %+v, want orange", item.Appearance)
	}
	if p := item.Primary; p == nil || p.Kind != api.ActionSystem || p.Op != "restart" || p.Title != "Open" {
		t.Errorf("primary = %+v, want the restart op", p)
	}
}

func TestKeywordMatchRanksBelowTitleMatches(t *testing.T) {
	items := searchTitles(t, allSupported(), "reboot")
	if len(items) != 1 || items[0].Score != keywordMatchScore {
		t.Fatalf("items = %+v, want Restart at the keyword score", items)
	}
}

func TestDestructiveCommandsAskWithTheirQuestion(t *testing.T) {
	items := searchTitles(t, allSupported(), "trash")
	if len(items) != 1 {
		t.Fatalf("items = %+v, want Empty Trash", items)
	}
	confirm := items[0].Primary.Confirm
	if confirm == nil || confirm.Title != "Empty Trash?" || confirm.ConfirmLabel != "Empty Trash" || !confirm.Destructive {
		t.Fatalf("confirmation = %+v", confirm)
	}
	if confirm.Message != "Permanently delete the files in the Trash. This cannot be undone." {
		t.Errorf("message = %q", confirm.Message)
	}
}

func TestSleepDoesNotAsk(t *testing.T) {
	items := searchTitles(t, allSupported(), "sleep")
	if len(items) != 1 || items[0].Primary.Confirm != nil {
		t.Fatalf("items = %+v, want Sleep without a confirmation", items)
	}
}

func TestCommandsTheSystemLacksAreOmitted(t *testing.T) {
	supported := map[string]bool{"system.sleep": true}
	if items := searchTitles(t, supported, "lock"); len(items) != 0 {
		t.Fatalf("items = %+v, want no lock command without its capability", items)
	}
	if items := searchTitles(t, supported, "sleep"); len(items) != 1 {
		t.Fatalf("items = %+v, want Sleep", items)
	}
}

func TestSettingsCommandUsesTheOSLabel(t *testing.T) {
	items := searchTitles(t, allSupported(), "preferences")
	if len(items) != 1 || items[0].Title != "Open System Settings" || items[0].Primary.Op != "open-system-settings" {
		t.Fatalf("items = %+v, want the settings command", items)
	}
}

// TestRootListsLockSleepRestartAndSettingsAtTheirOneXScores pins the commands the empty root shows: the four 1.x
// commands in their order, and none of the commands only a query finds.
func TestRootListsLockSleepRestartAndSettingsAtTheirOneXScores(t *testing.T) {
	items, err := Extension{}.RootItems(fakeContext{supported: allSupported()})
	if err != nil {
		t.Fatalf("RootItems: %v", err)
	}
	wantTitles := []string{"Lock Screen", "Sleep", "Restart Computer"}
	wantScores := []float64{22, 21, 20, 19}
	if len(items) != len(wantScores) {
		t.Fatalf("root lists %d commands, want %d", len(items), len(wantScores))
	}
	for i, want := range wantScores {
		if items[i].Score != want {
			t.Fatalf("row %d scores %v, want %v", i, items[i].Score, want)
		}
		if i < len(wantTitles) && items[i].Title != wantTitles[i] {
			t.Fatalf("row %d is %q, want %q", i, items[i].Title, wantTitles[i])
		}
	}
}
