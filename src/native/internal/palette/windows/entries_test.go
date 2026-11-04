package windows

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/commands"
)

func entryByID(t *testing.T, entries []Entry, id string) Entry {
	t.Helper()
	for _, entry := range entries {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("no entry %q", id)
	return Entry{}
}

func TestEntriesCoverEveryCatalogCommandAndTheViews(t *testing.T) {
	entries := Entries(newDesk(t).deps)
	want := len(commands.All()) + viewCount
	if len(entries) != want {
		t.Fatalf("%d entries, want %d", len(entries), want)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.ID] {
			t.Errorf("duplicate entry %q", entry.ID)
		}
		seen[entry.ID] = true
		if entry.Run == nil || entry.Title == "" || entry.Subtitle == "" {
			t.Errorf("entry %q is incomplete: %+v", entry.ID, entry)
		}
		if !entry.Hidden {
			t.Errorf("entry %q shows on the empty query; window commands answer typed queries only", entry.ID)
		}
	}
}

func TestEntryIDsAreTheShortcutNames(t *testing.T) {
	entries := Entries(newDesk(t).deps)
	for _, id := range []string{"window.left-half", "window.focus-left", switchEntryID, settingsEntryID, customEntryID} {
		entryByID(t, entries, id)
	}
}

func TestLayoutEntryWithoutATargetNotifiesThePalette(t *testing.T) {
	d := newDesk(t)
	d.deps.Target = func() (windowmgmt.Target, bool) { return windowmgmt.Target{}, false }
	entry := entryByID(t, Entries(d.deps), "window.left-half")
	entry.Run(&recordingHost{})
	note := d.awaitNote(t)
	if note[0] != "Left Half" || !strings.HasPrefix(note[1], "No target app was captured.") {
		t.Errorf("notification = %q", note)
	}
}

func TestLayoutEntryThatReachesItsWindowSaysNothing(t *testing.T) {
	d := newDesk(t)
	entry := entryByID(t, Entries(d.deps), "window.right-half")
	entry.Run(&recordingHost{})
	select {
	case note := <-d.notes:
		t.Fatalf("unexpected notification %q", note)
	default:
	}
}
