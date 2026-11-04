package places

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeContext serves the System service with a fixed list of places; every other service is absent.
type fakeContext struct {
	api.Context
	places []api.Place
}

func (c fakeContext) System() api.System { return fakeSystem{places: c.places} }

type fakeSystem struct {
	api.System
	places []api.Place
}

func (s fakeSystem) Places() []api.Place { return s.places }

var home = []api.Place{
	{Name: "Downloads", Path: "/home/alice/Downloads"},
	{Name: "Documents", Path: "/home/alice/Documents"},
	{Name: "Desktop", Path: "/home/alice/Desktop"},
	{Name: "Pictures", Path: "/home/alice/Pictures"},
}

func searchPlaces(t *testing.T, query string) []api.Item {
	t.Helper()
	items, err := Extension{}.SearchItems(fakeContext{places: home}, query)
	if err != nil {
		t.Fatalf("SearchItems(%q): %v", query, err)
	}
	return items
}

func TestEmptyQueryListsNoPlaces(t *testing.T) {
	if items := searchPlaces(t, ""); len(items) != 0 {
		t.Fatalf("items = %+v, want none for the empty root", items)
	}
}

func TestNameMatchOpensTheFolder(t *testing.T) {
	items := searchPlaces(t, "downloads")
	if len(items) != 1 || items[0].Title != "Downloads" {
		t.Fatalf("items = %+v, want Downloads", items)
	}
	item := items[0]
	if item.Subtitle != "/home/alice/Downloads" || item.Icon != "folder" || item.LearnKey != "cmd:Downloads" || item.Section != "Commands" {
		t.Errorf("row = %+v", item)
	}
	if item.Appearance == nil || item.Appearance.Foreground != api.ColorBlue {
		t.Errorf("appearance = %+v, want blue", item.Appearance)
	}
	if p := item.Primary; p == nil || p.Kind != api.ActionOpenPath || p.Path != "/home/alice/Downloads" || p.Title != "Open Downloads" {
		t.Errorf("primary = %+v, want an open-path action", p)
	}
}

func TestKeywordMatchesListEveryFolder(t *testing.T) {
	if items := searchPlaces(t, "open"); len(items) != len(home) {
		t.Fatalf("items = %+v, want all four folders for the keyword", items)
	}
}

func TestSingleLetterDoesNotMatchKeywords(t *testing.T) {
	if items := searchPlaces(t, "g"); len(items) != 0 {
		t.Fatalf("items = %+v, want none for a one-letter keyword query", items)
	}
}

func TestNoHomeMeansNoPlaces(t *testing.T) {
	items, err := Extension{}.SearchItems(fakeContext{}, "downloads")
	if err != nil || len(items) != 0 {
		t.Fatalf("items = %+v, %v; want none without a home", items, err)
	}
}
