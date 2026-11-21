package manager

import (
	"slices"
	"testing"
)

// TestCatalogListsDisabledExtensionsWithTheirRowTitles: the builder sees every installed folder, on or off, and the row
// titles read from each source, so a disabled converted toggle still counts as installed.
func TestCatalogListsDisabledExtensionsWithTheirRowTitles(t *testing.T) {
	h := newHarness(t)
	h.placeFolder(t, "acme.theme")
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	catalog := h.m.Catalog()
	if len(catalog) != 1 {
		t.Fatalf("catalog = %+v; want the one installed folder", catalog)
	}
	entry := catalog[0]
	if entry.ID != "acme.theme" || entry.Enabled {
		t.Fatalf("entry = %+v; want acme.theme, off", entry)
	}
	if !slices.Contains(entry.Titles, "Toggle System Theme") {
		t.Fatalf("titles = %q; want the command's row title", entry.Titles)
	}
}
