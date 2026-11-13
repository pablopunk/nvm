package notes

import (
	"strings"
	"testing"
)

// A search needs every word, ignores case, puts title hits first and keeps recency inside each group.
func TestSearchRanksTitleHitsFirstThenRecency(t *testing.T) {
	f := newFixture(t)
	body := f.newNote("Trip\nremember the packing list for Lisbon")
	f.set.Clock.Advance(1)
	title := f.newNote("# Packing list\nsocks")
	f.set.Clock.Advance(1)
	f.newNote("Groceries\nmilk")
	got := f.notes.Search("PACKING list", 10)
	if len(got) != 2 || got[0].Note.ID != title.ID || got[1].Note.ID != body.ID {
		t.Fatalf("search = %+v; want the title hit then the body hit", got)
	}
	if got[0].Title != "Packing list" || !strings.Contains(got[1].Snippet, "packing list") {
		t.Fatalf("title %q, snippet %q", got[0].Title, got[1].Snippet)
	}
}

func TestSearchWithoutQueryListsTheNewestUpToTheLimit(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 5; i++ {
		f.newNote("note")
		f.set.Clock.Advance(1)
	}
	if got := f.notes.Search("", 3); len(got) != 3 {
		t.Fatalf("search returned %d notes; want the limit of 3", len(got))
	}
	if got := f.notes.Search("", 0); len(got) != 5 {
		t.Fatalf("a zero limit returned %d notes; want the default to show all 5", len(got))
	}
	if got := f.notes.Search("absent", 5); len(got) != 0 {
		t.Fatalf("search for a missing word found %+v", got)
	}
}

func TestCreateWithStoresTheText(t *testing.T) {
	f := newFixture(t)
	note, err := f.notes.CreateWith(f.set.Clock.Now(), "# Plan\nstep one")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := f.notes.Get(note.ID)
	if !ok || got.Markdown != "# Plan\nstep one" {
		t.Fatalf("stored note = %+v, %v", got, ok)
	}
}
