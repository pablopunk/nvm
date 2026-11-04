package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

const clipboardCommandTitle = "Clipboard History"

// shownPalette is goldenPalette attached to a visible window, so the search field has focus and Enter reaches the rows.
func shownPalette(t *testing.T) *App {
	t.Helper()
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	p.Show()
	return p
}

// rootRowTitled returns the index and the row titled title among the rows on screen.
func rootRowTitled(t *testing.T, p *App, title string) (int, search.Item) {
	t.Helper()
	for i, row := range p.Results() {
		if row.Title == title {
			return i, row
		}
	}
	t.Fatalf("no root row titled %q among %d rows", title, len(p.Results()))
	return 0, search.Item{}
}

func searchActionTitles(actions []search.Action) []string {
	titles := make([]string, 0, len(actions))
	for _, action := range actions {
		titles = append(titles, action.Title)
	}
	return titles
}

// openClipboardCommandRow searches the root for the Clipboard History command and chooses its row.
func openClipboardCommandRow(t *testing.T, p *App) search.Item {
	t.Helper()
	goldenQuery(p, clipboardCommandTitle)
	i, row := rootRowTitled(t, p, clipboardCommandTitle)
	p.Choose(i)
	return row
}

// rootRowTitledAfterSearch searches the root for title and returns the row's index and item.
func rootRowTitledAfterSearch(t *testing.T, p *App, title string) (int, search.Item) {
	t.Helper()
	goldenQuery(p, title)
	return rootRowTitled(t, p, title)
}

// searchedClipboardCommandRow searches the root again, so the rows show the shortcuts saved since the last search.
func searchedClipboardCommandRow(t *testing.T, p *App) search.Item {
	t.Helper()
	goldenQuery(p, clipboardCommandTitle)
	_, row := rootRowTitled(t, p, clipboardCommandTitle)
	return row
}

// TestRootCommandRowOffersSetAndRemoveKeyboardShortcut checks the action menu of a root command: an unbound command
// offers Set keyboard shortcut only, and a bound one also offers Remove keyboard shortcut and shows its shortcut.
func TestRootCommandRowOffersSetAndRemoveKeyboardShortcut(t *testing.T) {
	p := goldenPalette(t)
	row := openClipboardCommandRow(t, p)
	if got, want := searchActionTitles(row.Actions), []string{"Open", setShortcutTitle}; !slices.Equal(got, want) {
		t.Fatalf("unbound actions = %v, want %v", got, want)
	}
	mustSetShortcut(t, p.shortcutsManager(), shortcuts.CommandClipboardHistory, "Command+Alt+V")
	row = searchedClipboardCommandRow(t, p)
	if got, want := searchActionTitles(row.Actions), []string{"Open", setShortcutTitle, removeShortcutTitle}; !slices.Equal(got, want) {
		t.Fatalf("bound actions = %v, want %v", got, want)
	}
	if row.Accessory != shortcutLabelOf("Command+Alt+V") {
		t.Fatalf("accessory = %q, want the bound shortcut", row.Accessory)
	}
}

// TestCommandKSetShortcutRecordsAndBindsFromTheRootRow goes through the keys: Command-K on the command opens its menu,
// Down moves to Set keyboard shortcut, Enter opens the recorder, and the pressed shortcut is saved on Enter.
func TestCommandKSetShortcutRecordsAndBindsFromTheRootRow(t *testing.T) {
	p := shownPalette(t)
	openClipboardCommandRow(t, p)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := p.Top().Title; got != "Record Shortcut" {
		t.Fatalf("top view = %q, want the recorder; chosen row %q of %d", got, p.Results()[p.Chosen()].Title, len(p.Results()))
	}
	tt.Key(ui.Super|ui.Alt, ui.KeyV)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := entryOf(t, p, shortcuts.CommandClipboardHistory).Accelerator; got != "Command+Alt+V" {
		t.Fatalf("saved accelerator = %q, want Command+Alt+V", got)
	}
	if got := p.Top().Title; got != rootViewTitle {
		t.Fatalf("after saving the top view = %q, want the root", got)
	}
	if row := searchedClipboardCommandRow(t, p); row.Accessory != shortcutLabelOf("Command+Alt+V") {
		t.Fatalf("row accessory = %q after saving", row.Accessory)
	}
}

// TestRemoveKeyboardShortcutFromTheRootRowUnbindsIt runs the Remove keyboard shortcut row of a bound command.
func TestRemoveKeyboardShortcutFromTheRootRowUnbindsIt(t *testing.T) {
	p := shownPalette(t)
	mustSetShortcut(t, p.shortcutsManager(), shortcuts.CommandClipboardHistory, "Command+Alt+V")
	row := openClipboardCommandRow(t, p)
	row.Actions[2].Run(p)
	if got := entryOf(t, p, shortcuts.CommandClipboardHistory).Accelerator; got != "" {
		t.Fatalf("accelerator after remove = %q", got)
	}
	if row := searchedClipboardCommandRow(t, p); row.Accessory != "" || len(row.Actions) != 2 {
		t.Fatalf("row after remove: accessory %q, %d actions", row.Accessory, len(row.Actions))
	}
}

// TestShortcutActionOfACommandOpensTheCommand resolves the action a shortcut runs for a command row and runs it.
func TestShortcutActionOfACommandOpensTheCommand(t *testing.T) {
	p := goldenPalette(t)
	action, ok := p.shortcutAction(shortcuts.CommandClipboardHistory)
	if !ok {
		t.Fatal("no action for the Clipboard History shortcut")
	}
	action.Run(p)
	if got := p.Top().Title; got != "Clipboard History" {
		t.Fatalf("top view = %q, want the clipboard history", got)
	}
}

// TestPaletteRootRowActionMenuShowsTheShortcutActions is the action menu of a command with and without a shortcut.
func TestPaletteRootRowActionMenuShowsTheShortcutActions(t *testing.T) {
	p := goldenPalette(t)
	openClipboardCommandRow(t, p)
	p.OpenActionMenu(0)
	golden.AssertMatrix(t, "root-row-actions", p.View, paletteGoldens)
}

func TestPaletteRootRowActionMenuShowsTheRemoveShortcutAction(t *testing.T) {
	p := goldenPalette(t)
	mustSetShortcut(t, p.shortcutsManager(), shortcuts.CommandClipboardHistory, "Command+Alt+V")
	openClipboardCommandRow(t, p)
	p.OpenActionMenu(0)
	golden.AssertMatrix(t, "root-row-actions-shortcut", p.View, paletteGoldens)
}
