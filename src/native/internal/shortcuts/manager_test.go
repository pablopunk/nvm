package shortcuts

import (
	"reflect"
	"slices"
	"testing"
)

var testCommands = []Action{
	{ID: "command:clipboard", Title: "Clipboard History"},
	{ID: "command:notes", Title: "Quick Notes"},
}

type managerFixture struct {
	hotkeys  *recordingHotkeys
	doc      *MemoryDocument
	manager  *Manager
	ran      []string
	palettes int
}

func newManagerFixture(t *testing.T, goos string, doc Document, declared func() []Declared, refused ...string) *managerFixture {
	t.Helper()
	f := &managerFixture{hotkeys: newRecordingHotkeys(refused...), doc: NewMemoryDocument(doc)}
	f.manager = NewManager(Setup{
		Doc:       f.doc,
		Hotkeys:   f.hotkeys,
		GOOS:      goos,
		Commands:  testCommands,
		Declared:  declared,
		Run:       func(actionID string) { f.ran = append(f.ran, actionID) },
		OnPalette: func() { f.palettes++ },
	})
	return f
}

func (f *managerFixture) bound() []string {
	keys := make([]string, 0, len(f.hotkeys.bound))
	for accelerator := range f.hotkeys.bound {
		keys = append(keys, accelerator)
	}
	slices.Sort(keys)
	return keys
}

func TestStartBindsThePaletteAndTheStoredActions(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:notes"] = "Command+Alt+N"
	f := newManagerFixture(t, "darwin", doc, nil)
	status := f.manager.Start()
	if !status.Registered || status.Accelerator != "Alt+Shift+Space" {
		t.Fatalf("palette status = %+v", status)
	}
	if want := []string{"Alt+Shift+Space", "Command+Alt+N"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
	f.hotkeys.press("Alt+Shift+Space")
	if f.palettes != 1 {
		t.Fatal("the palette key did not open Nevermind")
	}
}

func TestAnEmptyStoredPaletteFallsBackToTheDefault(t *testing.T) {
	f := newManagerFixture(t, "darwin", Document{}, nil)
	if status := f.manager.Start(); status.Accelerator != DefaultPalette {
		t.Fatalf("palette = %q, want %q", status.Accelerator, DefaultPalette)
	}
}

func TestSettingAnActionBindsItAndPressingItRunsIt(t *testing.T) {
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil)
	f.manager.Start()
	if err := f.manager.SetAction("command:clipboard", "cmd+option+v"); err != nil {
		t.Fatal(err)
	}
	f.hotkeys.press("Command+Alt+V")
	if want := []string{"command:clipboard"}; !reflect.DeepEqual(f.ran, want) {
		t.Fatalf("ran = %v, want %v", f.ran, want)
	}
	entry := entryFor(t, f.manager, "command:clipboard")
	if entry.Accelerator != "Command+Alt+V" || !entry.Removable || entry.Source != SourceUser {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestAConflictingActionShortcutIsRefusedAndNotSaved(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:clipboard"] = "Command+Alt+V"
	f := newManagerFixture(t, "darwin", doc, nil)
	f.manager.Start()
	err := f.manager.SetAction("command:notes", "Command+Alt+V")
	if err == nil || err.Error() != "Command+Alt+V is already used by Clipboard History" {
		t.Fatalf("error = %v", err)
	}
	if stored, _ := f.doc.Load(); stored.Actions["command:notes"] != "" {
		t.Fatal("a refused shortcut was saved")
	}
}

func TestTheOSRefusalShowsUnderTheRowAndKeepsTheChoice(t *testing.T) {
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil, "Command+Alt+N")
	f.manager.Start()
	if err := f.manager.SetAction("command:notes", "Command+Alt+N"); err != nil {
		t.Fatal(err)
	}
	entry := entryFor(t, f.manager, "command:notes")
	if entry.Accelerator != "Command+Alt+N" || entry.Inactive != "Command+Alt+N is in use by another app" {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestRemovingAnActionShortcutUnbindsIt(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:notes"] = "Command+Alt+N"
	f := newManagerFixture(t, "darwin", doc, nil)
	f.manager.Start()
	if err := f.manager.RemoveAction("command:notes"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alt+Shift+Space"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
	if entryFor(t, f.manager, "command:notes").Accelerator != "" {
		t.Fatal("a removed shortcut still shows")
	}
}

func TestSuspendReleasesEverythingAndResumeBindsItAgain(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:notes"] = "Command+Alt+N"
	f := newManagerFixture(t, "darwin", doc, nil)
	f.manager.Start()
	f.manager.Suspend()
	if len(f.bound()) != 0 {
		t.Fatalf("bound while suspended: %v", f.bound())
	}
	if err := f.manager.SetAction("command:clipboard", "Command+Alt+V"); err != nil {
		t.Fatal(err)
	}
	if len(f.bound()) != 0 {
		t.Fatalf("a save while suspended bound %v", f.bound())
	}
	f.manager.Resume()
	if want := []string{"Alt+Shift+Space", "Command+Alt+N", "Command+Alt+V"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound after resume = %v, want %v", f.bound(), want)
	}
}

func TestChangingThePaletteKeyBindsTheNewOneAndSavesIt(t *testing.T) {
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil)
	f.manager.Start()
	change, err := f.manager.SetPalette("ctrl+shift+space")
	if err != nil {
		t.Fatal(err)
	}
	if change.Accelerator != "Control+Shift+Space" || change.Unchanged {
		t.Fatalf("change = %+v", change)
	}
	if stored, _ := f.doc.Load(); stored.Palette != "Control+Shift+Space" {
		t.Fatalf("stored palette = %q", stored.Palette)
	}
	if want := []string{"Control+Shift+Space"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
}

func TestATakenPaletteKeyKeepsThePreviousOneBound(t *testing.T) {
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil, "Control+Shift+Space")
	f.manager.Start()
	if _, err := f.manager.SetPalette("ctrl+shift+space"); err == nil || err.Error() != "Control+Shift+Space is in use by another app" {
		t.Fatalf("error = %v", err)
	}
	if want := []string{"Alt+Shift+Space"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
	if stored, _ := f.doc.Load(); stored.Palette != "Alt+Shift+Space" {
		t.Fatalf("stored palette = %q after a refused change", stored.Palette)
	}
}

func TestSamePaletteKeyIsUnchangedAndSpotlightIsFlagged(t *testing.T) {
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), nil)
	f.manager.Start()
	if change, _ := f.manager.SetPalette("alt+shift+space"); !change.Unchanged {
		t.Fatalf("same key change = %+v", change)
	}
	change, err := f.manager.SetPalette("Command+Space")
	if err != nil || change.SystemOwner != "Spotlight" {
		t.Fatalf("spotlight change = %+v, %v", change, err)
	}
}

func TestOnLinuxAPalettePickSurvivesWithARecoveryKey(t *testing.T) {
	f := newManagerFixture(t, "linux", DefaultDocument("Alt+Shift+Space"), nil, "Control+Shift+Space")
	f.manager.Start()
	change, err := f.manager.SetPalette("ctrl+shift+space")
	if err != nil {
		t.Fatal(err)
	}
	if change.Recovery != "Control+Alt+K" {
		t.Fatalf("recovery = %q", change.Recovery)
	}
}

func TestDeclaredShortcutsShowUntilTheUserChoosesOrRemovesThem(t *testing.T) {
	declared := func() []Declared {
		return []Declared{{ActionID: "ext:snip", Accelerator: "Command+Alt+S", Title: "Insert Snippet"}}
	}
	f := newManagerFixture(t, "darwin", DefaultDocument("Alt+Shift+Space"), declared)
	f.manager.Start()
	if want := []string{"Alt+Shift+Space", "Command+Alt+S"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
	if err := f.manager.RemoveAction("ext:snip"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alt+Shift+Space"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound after removal = %v, want %v", f.bound(), want)
	}
	if entry := entryFor(t, f.manager, "ext:snip"); entry.Accelerator != "" || entry.Removable {
		t.Fatalf("removed declared entry = %+v", entry)
	}
}

func entryFor(t *testing.T, m *Manager, actionID string) Entry {
	t.Helper()
	for _, entry := range m.Entries() {
		if entry.ActionID == actionID {
			return entry
		}
	}
	t.Fatalf("no entry for %s in %+v", actionID, m.Entries())
	return Entry{}
}
