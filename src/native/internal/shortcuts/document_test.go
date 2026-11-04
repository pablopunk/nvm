package shortcuts

import (
	"errors"
	"reflect"
	"testing"
)

// titles names the actions in conflict messages, as the list does.
func titles(id string) string {
	return map[string]string{"clipboard": "Clipboard History", "notes": "Quick Notes"}[id]
}

// The conflict table of a user's choice: each row is an edit of the same document and the outcome the palette shows.
func TestSettingAShortcutFollowsTheConflictTable(t *testing.T) {
	base := DefaultDocument("Alt+Shift+Space")
	base.Actions["clipboard"] = "Command+Alt+V"
	tests := []struct {
		name        string
		actionID    string
		accelerator string
		reserved    func(string) bool
		wantErr     string
		wantStored  string
	}{
		{"new shortcut", "notes", "cmd+option+n", noneReserved, "", "Command+Alt+N"},
		{"wording is normalized", "notes", " ctrl + shift + n ", noneReserved, "", "Control+Shift+N"},
		{"blank is missing", "notes", "   ", noneReserved, ErrMissingShortcut.Error(), ""},
		{"palette key is reserved", "notes", "Alt+Shift+Space", func(a string) bool { return a == "Alt+Shift+Space" }, "Alt+Shift+Space is reserved for opening Nevermind", ""},
		{"another action's key is taken", "notes", "Command+Alt+V", noneReserved, "Command+Alt+V is already used by Clipboard History", ""},
		{"same action keeps its key", "clipboard", "Command+Alt+V", noneReserved, "", "Command+Alt+V"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := base.WithAction(tt.actionID, tt.accelerator, tt.reserved, titles)
			if !errMatches(err, tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if err != nil {
				if !reflect.DeepEqual(next, base) {
					t.Fatal("a rejected edit changed the document")
				}
				return
			}
			if got := next.Actions[tt.actionID]; got != tt.wantStored {
				t.Fatalf("stored = %q, want %q", got, tt.wantStored)
			}
		})
	}
}

func TestSettingAShortcutRevivesARemovedDeclaredOne(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Disabled["notes"] = true
	next, err := doc.WithAction("notes", "Command+Alt+N", noneReserved, titles)
	if err != nil {
		t.Fatal(err)
	}
	if next.Disabled["notes"] {
		t.Fatal("a chosen shortcut kept the removed marker")
	}
}

func TestRemovingAUserChoiceForgetsItAndAMissingOneFails(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["clipboard"] = "Command+Alt+V"
	next, err := doc.WithoutAction("clipboard", false)
	if err != nil || len(next.Actions) != 0 {
		t.Fatalf("remove = %+v, %v", next, err)
	}
	if _, err := doc.WithoutAction("notes", false); !errors.Is(err, ErrNoShortcut) {
		t.Fatalf("missing shortcut error = %v", err)
	}
}

func TestRemovingADeclaredShortcutMarksItOff(t *testing.T) {
	next, err := DefaultDocument("Alt+Shift+Space").WithoutAction("ext:thing", true)
	if err != nil {
		t.Fatal(err)
	}
	if !next.Disabled["ext:thing"] {
		t.Fatal("declared removal did not leave the marker")
	}
}

func TestPaletteHotkeyRefusesAUserShortcutAndAcceptsOthers(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	doc.Actions["clipboard"] = "Command+Alt+V"
	if _, err := doc.WithPalette("cmd+option+v", titles); err == nil || err.Error() != "Command+Alt+V is already used by Clipboard History" {
		t.Fatalf("conflicting palette error = %v", err)
	}
	next, err := doc.WithPalette("ctrl+shift+space", titles)
	if err != nil || next.Palette != "Control+Shift+Space" {
		t.Fatalf("palette = %q, %v", next.Palette, err)
	}
	if doc.Palette != "Alt+Shift+Space" {
		t.Fatal("WithPalette changed the receiver")
	}
}

func TestEditsDoNotShareMapsWithTheReceiver(t *testing.T) {
	doc := DefaultDocument("Alt+Shift+Space")
	next, _ := doc.WithAction("notes", "Command+Alt+N", noneReserved, titles)
	if len(doc.Actions) != 0 || len(next.Actions) != 1 {
		t.Fatal("an edit leaked into the document it started from")
	}
}

func errMatches(err error, want string) bool {
	if want == "" {
		return err == nil
	}
	return err != nil && err.Error() == want
}
