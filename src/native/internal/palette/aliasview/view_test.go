package aliasview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
)

type fakeStore struct {
	aliases map[string][]string
}

func (s *fakeStore) For(id string) []string { return slices.Clone(s.aliases[id]) }

func (s *fakeStore) Add(id, alias string) error {
	s.aliases[id] = append(s.aliases[id], alias)
	return nil
}

func (s *fakeStore) Remove(id, alias string) error {
	s.aliases[id] = slices.DeleteFunc(s.aliases[id], func(v string) bool { return v == alias })
	return nil
}

// fakeHost is a search.Host that records only Pop; any other call panics, which a Done row never makes.
type fakeHost struct {
	search.Host
	popped bool
}

func (h *fakeHost) Pop() { h.popped = true }

func titlesOf(items []search.Item) []string {
	var out []string
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}

// TestRowsOfTheAliasViewSaveRemoveAndFinish checks the rows 1.x shows: a save row for a new phrase, one row per saved
// alias, and Done last.
func TestRowsOfTheAliasViewSaveRemoveAndFinish(t *testing.T) {
	store := &fakeStore{aliases: map[string][]string{"command:x": {"first"}}}
	target := Target{ID: "command:x", Title: "Example"}
	got := titlesOf(rows(target, store, "second"))
	if want := []string{"Save alias “second”", "first", "Done"}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if got := titlesOf(rows(target, store, "first")); !slices.Equal(got, []string{"first", "Done"}) {
		t.Fatalf("a saved phrase offers no save row: %v", got)
	}
}

// TestDoneLeavesTheAliasView checks that Done pops the view.
func TestDoneLeavesTheAliasView(t *testing.T) {
	store := &fakeStore{aliases: map[string][]string{}}
	items := rows(Target{ID: "command:x", Title: "Example"}, store, "")
	host := &fakeHost{}
	items[len(items)-1].Actions[0].Run(host)
	if !host.popped {
		t.Fatal("Done did not leave the alias view")
	}
}
