package shortcuts

import "testing"

func TestEntriesListEveryCommandSortedByTitleWithItsShortcut(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["command:notes"] = "Command+Alt+N"
	entries := Entries(doc, testCommands, nil, map[string]string{"command:notes": "Command+Alt+N is in use by another app"})
	var titles []string
	for _, entry := range entries {
		titles = append(titles, entry.Title)
	}
	want := []string{"Clipboard History", "Floating Notes", "Palette Hotkey"}
	if !equalStrings(titles, want) {
		t.Fatalf("titles = %v, want %v", titles, want)
	}
	notes := entries[1]
	if notes.Accelerator != "Command+Alt+N" || notes.Inactive == "" || !notes.Removable {
		t.Fatalf("notes entry = %+v", notes)
	}
	if entries[0].Accelerator != "" {
		t.Fatalf("clipboard entry = %+v, want no shortcut", entries[0])
	}
}

func TestEntriesShowADeclaredShortcutAsDeclaredAndNotRemovable(t *testing.T) {
	declared := []Declared{{ActionID: "ext:x", Accelerator: "cmd+x", Title: "Extension X"}}
	entries := Entries(DefaultDocument("Alt+Shift+Space"), nil, declared, nil)
	var found Entry
	for _, entry := range entries {
		if entry.ActionID == "ext:x" {
			found = entry
		}
	}
	if found.Accelerator != "Command+X" || found.Source != SourceDeclared || found.Removable {
		t.Fatalf("declared entry = %+v", found)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
