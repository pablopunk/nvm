package app

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// inventoryPath is the Command-K parity inventory of 1.x actions, read from the repository.
var inventoryPath = filepath.Join("..", "..", "testdata", "parity", "cmdk-actions.md")

// menuCase is one row kind: how to reach its row, and the titles of its menu in 1.x order. Each title must also be named
// in the inventory, so the inventory stays the reference the menus are checked against.
type menuCase struct {
	kind  string
	row   func(t *testing.T) search.Item
	menu  []string
}

// menuCases lists the row kinds the palette can reach from the golden fixtures.
func menuCases() []menuCase {
	return []menuCase{
		{kind: "file", row: func(t *testing.T) search.Item {
			svc := newTestFiles(t, testFileHome, fixturePaths())
			return svc.fileItem(files.Result{Path: fixturePaths()[0], Name: "report-2024.pdf", DisplayPath: "~/Documents/report-2024.pdf"})
		}, menu: []string{"Open report-2024.pdf", "Preview Contents", platform.ShellLabelsFor(runtime.GOOS).RevealInFileManager}},
		{kind: "clipboard entry", row: func(t *testing.T) search.Item {
			p, _, _ := seededClipPalette(t)
			return p.Results()[1]
		}, menu: []string{"Copy Text", "Preview", "Paste Text", "Remove Item", "Remove Last..."}},
		{kind: "command", row: func(t *testing.T) search.Item {
			return openClipboardCommandRow(t, goldenPalette(t))
		}, menu: []string{clipboardCommandTitle, setShortcutTitle, setAliasTitle}},
		{kind: "system command", row: func(t *testing.T) search.Item {
			p := goldenPalette(t)
			_, row := rootRowTitledAfterSearch(t, p, "Lock Screen")
			return row
		}, menu: []string{"Lock Screen", setShortcutTitle, setAliasTitle}},
	}
}

// TestRowMenusMatchTheInventory checks the action menu of each row kind: its titles, in order, are the ones the
// inventory lists for that kind, and each title is named in the inventory file.
func TestRowMenusMatchTheInventory(t *testing.T) {
	data, err := os.ReadFile(inventoryPath)
	if err != nil {
		t.Fatalf("read the inventory: %v", err)
	}
	inventory := string(data)
	for _, c := range menuCases() {
		row := c.row(t)
		got := searchActionTitles(row.Actions)
		if !slices.Equal(got, c.menu) {
			t.Errorf("%s menu = %v, want %v", c.kind, got, c.menu)
		}
		for _, title := range c.menu {
			if !strings.Contains(inventory, title) && !inventoryNamesWithPlaceholder(inventory, title) {
				t.Errorf("%s menu title %q is not in the inventory", c.kind, title)
			}
		}
	}
}

// inventoryNamesWithPlaceholder reports whether the inventory names title with its name written as <name>, as the
// inventory writes row titles that carry the item's name.
func inventoryNamesWithPlaceholder(inventory, title string) bool {
	for _, name := range []string{"report-2024.pdf"} {
		if strings.Contains(title, name) && strings.Contains(inventory, strings.Replace(title, name, "<name>", 1)) {
			return true
		}
	}
	return false
}
