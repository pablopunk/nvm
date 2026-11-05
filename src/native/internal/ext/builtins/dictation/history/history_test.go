package history

import (
	"slices"
	"strings"
	"testing"
)

func TestAddPutsTheNewestTranscriptFirstAndCapsTheList(t *testing.T) {
	var entries []Entry
	for index := range MaxEntries + 3 {
		entries = Add(entries, Entry{ID: string(rune('a' + index%26)), Text: "text", CreatedAt: int64(index + 1)})
	}
	if len(entries) != MaxEntries || entries[0].CreatedAt != MaxEntries+3 {
		t.Fatalf("len %d, first %+v; want %d entries with the newest first", len(entries), entries[0], MaxEntries)
	}
}

func TestAddKeepsNoBlankTranscript(t *testing.T) {
	if got := Add(nil, Entry{ID: "x", Text: "  \n ", CreatedAt: 1}); len(got) != 0 {
		t.Fatalf("Add of blank text kept %+v", got)
	}
}

func TestRemoveDropsOnlyTheNamedEntryAndLeavesTheOriginal(t *testing.T) {
	original := []Entry{{ID: "a", Text: "one", CreatedAt: 2}, {ID: "b", Text: "two", CreatedAt: 1}}
	got := Remove(original, "a")
	if len(got) != 1 || got[0].ID != "b" || len(original) != 2 {
		t.Fatalf("Remove = %+v; original = %+v", got, original)
	}
}

func TestCleanDropsEntriesThatCannotBeShownAndSortsNewestFirst(t *testing.T) {
	got := Clean([]Entry{
		{ID: "old", Text: "a", CreatedAt: 1},
		{ID: "", Text: "no id", CreatedAt: 9},
		{ID: "blank", Text: " ", CreatedAt: 8},
		{ID: "new", Text: "b", CreatedAt: 5},
		{ID: "untimed", Text: "c"},
	})
	ids := []string{}
	for _, entry := range got {
		ids = append(ids, entry.ID)
	}
	if !slices.Equal(ids, []string{"new", "old"}) {
		t.Fatalf("Clean ids = %v, want the showable ones newest first", ids)
	}
}

func TestTitleCollapsesWhitespaceAndCutsToTheLimit(t *testing.T) {
	if got := Title("hello \n  there\tfriend"); got != "hello there friend" {
		t.Fatalf("Title = %q", got)
	}
	if got := Title(strings.Repeat("é", TitleChars+5)); len([]rune(got)) != TitleChars {
		t.Fatalf("Title length = %d runes, want %d", len([]rune(got)), TitleChars)
	}
}
