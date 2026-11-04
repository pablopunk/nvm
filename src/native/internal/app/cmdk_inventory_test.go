package app

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// inventoryPath is the Command-K parity inventory of 1.x actions, read from the repository.
var inventoryPath = filepath.Join("..", "..", "testdata", "parity", "cmdk-actions.md")

// menuCase is one row kind: how to reach its row, the name the row's titles carry (written as <name> in the inventory),
// and the titles of its menu in 1.x order. Each title must also be named in the inventory.
type menuCase struct {
	kind string
	name string
	row  func(t *testing.T) search.Item
	menu []string
	// skipPrimary leaves the first title out of the inventory check: the primary action is the command's own title, which
	// the inventory names as a rule, not by a fixed string.
	skipPrimary bool
}

// rootRowNamed searches the golden root for title and returns the row titled title.
func rootRowNamed(t *testing.T, title string) search.Item {
	t.Helper()
	_, row := rootRowTitledAfterSearch(t, goldenPalette(t), title)
	return row
}

// menuCases lists the row kinds the palette can reach from the golden fixtures and the file service.
func menuCases(t *testing.T) []menuCase {
	fileName := "report-2024.pdf"
	return []menuCase{
		{kind: "application", name: "Firefox", row: func(t *testing.T) search.Item {
			return rootRowNamed(t, "Firefox")
		}, menu: applicationMenu(goldenPalette(t).appsvc.SupportsUninstall())},
		{kind: "file", name: fileName, row: func(t *testing.T) search.Item {
			svc := newTestFiles(t, testFileHome, fixturePaths())
			return svc.fileItem(files.Result{Path: fixturePaths()[0], Name: fileName, DisplayPath: "~/Documents/" + fileName})
		}, menu: []string{"Open " + fileName, "Preview Contents", platform.ShellLabelsFor(runtime.GOOS).RevealInFileManager}},
		{kind: "clipboard entry", row: func(t *testing.T) search.Item {
			p, _, _ := seededClipPalette(t)
			return p.Results()[1]
		}, menu: []string{"Copy Text", "Preview", "Paste Text", "Remove Item", "Remove Last..."}},
		{kind: "calculator", row: func(t *testing.T) search.Item {
			p := goldenPalette(t)
			goldenQuery(p, "2+2")
			return p.Results()[0]
		}, menu: []string{"Copy Result", "Continue Calculation", "Paste Result"}},
		{kind: "command", row: func(t *testing.T) search.Item {
			return openClipboardCommandRow(t, goldenPalette(t))
		}, menu: []string{clipboardCommandTitle, setShortcutTitle, setAliasTitle}},
		{kind: "system command", row: func(t *testing.T) search.Item {
			return rootRowNamed(t, "Lock Screen")
		}, menu: []string{"Lock Screen", setShortcutTitle, setAliasTitle}},
		{kind: "folder", row: func(t *testing.T) search.Item {
			return rootRowNamed(t, "Downloads")
		}, menu: []string{"Open Downloads", setShortcutTitle, setAliasTitle}},
		{kind: "AI chat", name: "Trip ideas", row: func(t *testing.T) search.Item {
			deps := aichat.Deps{Now: func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) }}
			return aichat.ConversationRow(deps, chat.Chat{ID: "chat-trip", Title: "Trip ideas", UpdatedAt: time.Date(2026, 11, 4, 11, 0, 0, 0, time.UTC)})
		}, menu: []string{"Open", "Remove action"}},
		{kind: "extension command", row: func(t *testing.T) search.Item {
			return extensionCommandRow(t, goldenPalette(t), "")
		}, menu: []string{"New Note", setShortcutTitle, setAliasTitle, renameTitle, tweakWithAITitle}, skipPrimary: true},
	}
}

// extensionCommandRow returns the root row of an extension command that the aliases match for query. A test passes ""
// to get the row by its title, as the root list lists it.
func extensionCommandRow(t *testing.T, p *App, query string) search.Item {
	t.Helper()
	result := manager.CommandResult{ExtensionID: "acme.notes", Title: "Notes", Commands: []api.Command{{ID: "new", Title: "New Note"}}}
	rows := matchingCommandRows(p, p.extensionManager(), result, query, nil, p.aliasStore().All())
	if len(rows) != 1 {
		t.Fatalf("extension command rows for %q = %d, want 1", query, len(rows))
	}
	return rows[0]
}

// TestRowMenusMatchTheInventory checks the action menu of each row kind: its titles, in order, are the ones the inventory
// lists for that kind, and each title is named in the inventory file.
func TestRowMenusMatchTheInventory(t *testing.T) {
	data, err := os.ReadFile(inventoryPath)
	if err != nil {
		t.Fatalf("read the inventory: %v", err)
	}
	inventory := string(data)
	for _, c := range menuCases(t) {
		got := searchActionTitles(c.row(t).Actions)
		if !slices.Equal(got, c.menu) {
			t.Errorf("%s menu = %v, want %v", c.kind, got, c.menu)
		}
		for i, title := range c.menu {
			if c.skipPrimary && i == 0 {
				continue
			}
			if !namedInInventory(inventory, title, c.name) {
				t.Errorf("%s menu title %q is not named in the inventory", c.kind, title)
			}
		}
	}
}

// applicationMenu is the menu of an application row: Open, then Uninstall when the platform supports it, then the
// shortcut and alias actions.
func applicationMenu(uninstallable bool) []string {
	menu := []string{"Open Firefox"}
	if uninstallable {
		menu = append(menu, "Uninstall Firefox…")
	}
	return append(menu, setShortcutTitle, setAliasTitle)
}

// namedInInventory reports whether the inventory names title, with the item's name written as <name> when title carries it.
func namedInInventory(inventory, title, name string) bool {
	if strings.Contains(inventory, title) {
		return true
	}
	return name != "" && strings.Contains(title, name) && strings.Contains(inventory, strings.Replace(title, name, "<name>", 1))
}
