package shortcuts

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// queuedUI stands in for the UI thread. It runs each function at once, unless held, in which case the functions wait
// until drain runs them, as a busy UI thread would.
type queuedUI struct {
	held    bool
	waiting []func()
	ran     int
}

func (q *queuedUI) run(fn func()) {
	if q.held {
		q.waiting = append(q.waiting, fn)
		return
	}
	q.ran++
	fn()
}

func (q *queuedUI) drain() {
	for _, fn := range q.waiting {
		q.ran++
		fn()
	}
	q.waiting = nil
}

type editorFixture struct {
	*managerFixture
	editor  ExtensionEditor
	ui      *queuedUI
	changes int
}

func newEditorFixture(t *testing.T, refused ...string) *editorFixture {
	t.Helper()
	f := &editorFixture{managerFixture: newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil, refused...), ui: &queuedUI{}}
	f.manager.Start()
	table := NewExtensionTable(f.manager, func() Labeler { return Labeler{GOOS: "darwin"} }, NewAliases(NewMemoryAliases()))
	f.editor = NewExtensionEditor(table, f.ui.run, func() { f.changes++ })
	return f
}

func TestTheEditorListsThePaletteAndEveryCommand(t *testing.T) {
	f := newEditorFixture(t)
	records := f.editor.Bindable()
	if len(records) != 1+len(testCommands) || records[0].ActionID != PaletteID || records[0].Accelerator != "Alt+Shift+Space" {
		t.Fatalf("bindable = %+v", records)
	}
}

// Commands outside the list, such as an extension command with no shortcut yet, can be found and bound.
func TestTheEditorOffersCommandsTheSetupAddsAndBindsThem(t *testing.T) {
	f := &editorFixture{managerFixture: &managerFixture{hotkeys: newRecordingHotkeys(), doc: NewMemoryDocument(DefaultDocument("Alt+Shift+Space"))}, ui: &queuedUI{}}
	f.manager = NewManager(Setup{Doc: f.doc, Hotkeys: f.hotkeys, Clock: fake.NewClock(), GOOS: "darwin", Commands: testCommands,
		Bindable: func() []Action { return []Action{{ID: "ext:acme/hello", Title: "Say Hello"}} },
		Resolve:  func(id string) (string, bool) { return "Say Hello", id == "ext:acme/hello" },
		Run:      func(actionID string) { f.ran = append(f.ran, actionID) }, OnPalette: func() {}})
	f.manager.Start()
	editor := NewExtensionEditor(NewExtensionTable(f.manager, func() Labeler { return Labeler{GOOS: "darwin"} }, NewAliases(NewMemoryAliases())), f.ui.run, nil)
	if _, err := editor.Set(context.Background(), "ext:acme/hello", "Command+Alt+H", ""); err != nil {
		t.Fatal(err)
	}
	f.hotkeys.press("Command+Alt+H")
	if len(f.ran) != 1 || f.ran[0] != "ext:acme/hello" {
		t.Fatalf("ran = %v", f.ran)
	}
}

func TestTheEditorSetsAShortcutOnTheUIThread(t *testing.T) {
	f := newEditorFixture(t)
	record, err := f.editor.Set(context.Background(), "command:clipboard", "cmd+shift+v", "")
	if err != nil {
		t.Fatal(err)
	}
	if record.Accelerator != "Command+Shift+V" || record.Label != "⌘⇧V" || record.Title != "Clipboard History" {
		t.Fatalf("record = %+v", record)
	}
	if f.ui.ran != 1 || f.changes != 1 {
		t.Fatalf("ran on the UI thread %d times, after-change %d times", f.ui.ran, f.changes)
	}
	f.hotkeys.press("Command+Shift+V")
	if len(f.ran) != 1 || f.ran[0] != "command:clipboard" {
		t.Fatalf("ran = %v", f.ran)
	}
}

func TestTheEditorPlansWhatTheTableWouldDoAndRefusesWhatItCannot(t *testing.T) {
	f := newEditorFixture(t)
	change, err := f.editor.Plan("command:clipboard", "cmd+shift+v")
	if err != nil || change.Title != "Clipboard History" || change.Accelerator != "Command+Shift+V" || change.Label != "⌘⇧V" || change.Revision == "" {
		t.Fatalf("plan = %+v, %v", change, err)
	}
	refused := map[string][2]string{
		"unknown action":     {"command:unknown", "Command+K"},
		"palette key":        {"command:notes", "Alt+Shift+Space"},
		"palette removal":    {PaletteID, ""},
		"nothing to removal": {"command:notes", ""},
	}
	for name, call := range refused {
		if _, err := f.editor.Plan(call[0], call[1]); !errors.Is(err, api.ErrInvalidInput) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A change prepared on a table the user edited since is refused, so an approval never overwrites a newer choice.
func TestTheEditorRefusesAChangePreparedOnAnOlderTable(t *testing.T) {
	f := newEditorFixture(t)
	change, _ := f.editor.Plan("command:clipboard", "Command+Shift+V")
	if err := f.manager.SetAction("command:clipboard", "Control+Alt+V"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.editor.Set(context.Background(), "command:clipboard", "Command+Shift+V", change.Revision); !errors.Is(err, errTableMoved) {
		t.Fatalf("error = %v", err)
	}
	if stored, _ := f.doc.Load(); stored.Actions["command:clipboard"] != "Control+Alt+V" {
		t.Fatalf("stored = %v", stored.Actions)
	}
}

// A caller that gives up before the UI thread takes the change abandons it: draining the queue later saves nothing.
func TestAChangeAbandonedBeforeItStartsNeverRuns(t *testing.T) {
	f := newEditorFixture(t)
	f.ui.held = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.editor.Set(ctx, "command:clipboard", "Command+Shift+V", ""); !errors.Is(err, errChangeNotStarted) {
		t.Fatalf("error = %v", err)
	}
	f.ui.drain()
	if stored, _ := f.doc.Load(); stored.Actions["command:clipboard"] != "" || f.changes != 0 {
		t.Fatalf("an abandoned change was saved: %v", stored.Actions)
	}
}

// When the OS refuses the new key, the action keeps the shortcut that worked.
func TestAKeyTheOSRefusesKeepsThePreviousShortcut(t *testing.T) {
	f := newEditorFixture(t, "Command+Alt+N")
	if _, err := f.editor.Set(context.Background(), "command:notes", "Command+Alt+M", ""); err != nil {
		t.Fatal(err)
	}
	_, err := f.editor.Set(context.Background(), "command:notes", "Command+Alt+N", "")
	if err == nil || !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("error = %v", err)
	}
	if stored, _ := f.doc.Load(); stored.Actions["command:notes"] != "Command+Alt+M" {
		t.Fatalf("stored = %v; the working shortcut was lost", stored.Actions)
	}
	f.hotkeys.press("Command+Alt+M")
	if len(f.ran) != 1 || f.ran[0] != "command:notes" {
		t.Fatalf("ran = %v; the previous shortcut is not bound", f.ran)
	}
}

func TestTheEditorRemovesTheUsersShortcut(t *testing.T) {
	f := newEditorFixture(t)
	if _, err := f.editor.Set(context.Background(), "command:notes", "Command+Alt+N", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.editor.Remove(context.Background(), "command:notes", ""); err != nil {
		t.Fatal(err)
	}
	if stored, _ := f.doc.Load(); stored.Actions["command:notes"] != "" {
		t.Fatal("the removed shortcut is still stored")
	}
}
