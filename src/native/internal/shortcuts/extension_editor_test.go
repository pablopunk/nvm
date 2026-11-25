package shortcuts

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// queuedUI stands in for the UI thread: it runs each function at once and counts them, so a test sees every change
// went through it.
type queuedUI struct{ ran int }

func (q *queuedUI) run(fn func()) {
	q.ran++
	fn()
}

func newEditorFixture(t *testing.T) (*managerFixture, ExtensionEditor, *queuedUI, *int) {
	t.Helper()
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil)
	f.manager.Start()
	table := NewExtensionTable(f.manager, func() Labeler { return Labeler{GOOS: "darwin"} }, NewAliases(NewMemoryAliases()))
	ui := &queuedUI{}
	changes := 0
	return f, NewExtensionEditor(table, ui.run, func() { changes++ }), ui, &changes
}

func TestTheEditorListsThePaletteAndEveryCommand(t *testing.T) {
	_, editor, _, _ := newEditorFixture(t)
	records := editor.Bindable()
	if len(records) != 1+len(testCommands) || records[0].ActionID != PaletteID || records[0].Accelerator != "Alt+Shift+Space" {
		t.Fatalf("bindable = %+v", records)
	}
}

func TestTheEditorSetsAShortcutOnTheUIThread(t *testing.T) {
	f, editor, ui, changes := newEditorFixture(t)
	record, err := editor.Set(context.Background(), "command:clipboard", "cmd+shift+v")
	if err != nil {
		t.Fatal(err)
	}
	if record.Accelerator != "Command+Shift+V" || record.Label != "⌘⇧V" || record.Title != "Clipboard History" {
		t.Fatalf("record = %+v", record)
	}
	if ui.ran != 1 || *changes != 1 {
		t.Fatalf("ran on the UI thread %d times, after-change %d times", ui.ran, *changes)
	}
	f.hotkeys.press("Command+Shift+V")
	if len(f.ran) != 1 || f.ran[0] != "command:clipboard" {
		t.Fatalf("ran = %v", f.ran)
	}
}

func TestTheEditorRefusesWhatTheTableCannotTake(t *testing.T) {
	_, editor, ui, changes := newEditorFixture(t)
	if _, err := editor.Set(context.Background(), "command:unknown", "Command+K"); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("unknown action: %v", err)
	}
	if _, err := editor.Set(context.Background(), "command:notes", "Alt+Shift+Space"); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("palette accelerator: %v", err)
	}
	if err := editor.Remove(context.Background(), PaletteID); !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("palette remove: %v", err)
	}
	if *changes != 0 || ui.ran != 1 {
		t.Fatalf("a refused change counted: ui %d, changes %d", ui.ran, *changes)
	}
}

func TestTheEditorRemovesTheUsersShortcut(t *testing.T) {
	f, editor, _, _ := newEditorFixture(t)
	if _, err := editor.Set(context.Background(), "command:notes", "Command+Alt+N"); err != nil {
		t.Fatal(err)
	}
	if err := editor.Remove(context.Background(), "command:notes"); err != nil {
		t.Fatal(err)
	}
	if stored, _ := f.doc.Load(); stored.Actions["command:notes"] != "" {
		t.Fatal("the removed shortcut is still stored")
	}
}
