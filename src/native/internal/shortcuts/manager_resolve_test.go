package shortcuts

import (
	"reflect"
	"testing"
)

// knownApps resolves the app IDs a root row can carry; anything else is gone.
func knownApps(actionID string) (string, bool) {
	title, ok := map[string]string{"app:safari": "Safari"}[actionID]
	return title, ok
}

func newResolvingFixture(t *testing.T, doc Document) *managerFixture {
	t.Helper()
	f := newManagerFixture(t, "darwin", doc, nil)
	f.manager.setup.Resolve = knownApps
	return f
}

// A root row that is not in the command list, such as an app, takes a shortcut: it binds, a press runs it, and the
// Keyboard Shortcuts list shows it under its title.
func TestAResolvedActionBindsAndIsListedOnceChosen(t *testing.T) {
	f := newResolvingFixture(t, DefaultDocument("Alt+Shift+Space"))
	f.manager.Start()
	if err := f.manager.SetAction("app:safari", "Command+Alt+S"); err != nil {
		t.Fatal(err)
	}
	f.hotkeys.press("Command+Alt+S")
	if want := []string{"app:safari"}; !reflect.DeepEqual(f.ran, want) {
		t.Fatalf("ran = %v, want %v", f.ran, want)
	}
	entry := findEntry(t, f.manager.Entries(), "app:safari")
	if entry.Title != "Safari" || entry.Accelerator != "Command+Alt+S" || !entry.Removable {
		t.Fatalf("entry = %+v", entry)
	}
}

// A shortcut chosen for an action that is gone stays in the document but binds nothing and lists nothing.
func TestAnActionResolveNoLongerKnowsIsDormant(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["app:removed"] = "Command+Alt+R"
	f := newResolvingFixture(t, doc)
	f.manager.Start()
	if want := []string{"Alt+Shift+Space"}; !reflect.DeepEqual(f.bound(), want) {
		t.Fatalf("bound = %v, want %v", f.bound(), want)
	}
	for _, entry := range f.manager.Entries() {
		if entry.ActionID == "app:removed" {
			t.Fatalf("a gone action is listed: %+v", entry)
		}
	}
}

// Accelerators answers the root rows: the user's choices and the declared shortcuts that were not removed.
func TestAcceleratorsHoldsChoicesAndKeptDeclaredShortcuts(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:notes"] = "Command+Alt+N"
	doc.Disabled["ext:off"] = true
	declared := func() []Declared {
		return []Declared{{ActionID: "ext:on", Accelerator: "cmd+alt+o"}, {ActionID: "ext:off", Accelerator: "Command+Alt+F"}}
	}
	f := newManagerFixture(t, "darwin", doc, declared)
	want := map[string]string{"command:notes": "Command+Alt+N", "ext:on": "Command+Alt+O"}
	if got := f.manager.Accelerators(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Accelerators() = %v, want %v", got, want)
	}
}

func findEntry(t *testing.T, entries []Entry, actionID string) Entry {
	t.Helper()
	for _, entry := range entries {
		if entry.ActionID == actionID {
			return entry
		}
	}
	t.Fatalf("no entry for %s in %+v", actionID, entries)
	return Entry{}
}
