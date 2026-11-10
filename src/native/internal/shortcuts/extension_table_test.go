package shortcuts

import "testing"

// The extension list leaves the palette hotkey out, as 1.x's shortcuts.list() does. Palette() still reports it, as
// 1.x's shortcuts.palette() does.
func TestExtensionListLeavesThePaletteHotkeyOutAndPaletteReportsIt(t *testing.T) {
	f := newManagerFixture(t, "linux", DefaultDocument("Alt+Shift+Space"), nil)
	if err := f.manager.SetAction("command:notes", "Command+Alt+N"); err != nil {
		t.Fatal(err)
	}
	table := NewExtensionTable(f.manager, func() Labeler { return Labeler{GOOS: "darwin"} }, NewAliases(NewMemoryAliases()))
	records := table.List()
	if len(records) != 1 || records[0].ActionID != "command:notes" {
		t.Fatalf("extension list = %+v, want only Quick Notes", records)
	}
	if got := table.Palette().Accelerator; got != "Alt+Shift+Space" {
		t.Fatalf("palette accelerator = %q, want Alt+Shift+Space", got)
	}
}
