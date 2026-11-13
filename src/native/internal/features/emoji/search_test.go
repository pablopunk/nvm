package emoji

import (
	"strings"
	"testing"
)

// Each field matches the way 1.x's grid filter matches a value: it contains the query, or the query's words start the
// field's words in order or in any order.
func TestQueriesMatchFieldsAsThe1xGridFilterDoes(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		query string
		want  bool
	}{
		{"exact", "grinning face", "grinning face", true},
		{"starts with", "grinning", "grinn", true},
		{"contains inside a word", "xgrinningx", "grinning", true},
		{"contains a whole phrase", "a smiling face", "smiling face", true},
		{"words in order", "big grinning face", "grinning fa", true},
		{"words in any order", "grinning face", "face grin", true},
		{"scattered letters are no match", "grinning face", "gnf", false},
		{"no match", "grinning face", "zz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesQuery(tt.text, newSearchQuery(tt.query)); got != tt.want {
				t.Errorf("matchesQuery(%q, %q) = %v, want %v", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

// Scattered letters never match an emoji, in any field: "ecf" does not find "grinning face".
func TestScatteredLettersNeverMatchAnEmoji(t *testing.T) {
	if _, matched := rankRecord([]string{"grinning face"}, 1, newSearchQuery("ecf")); matched {
		t.Error("scattered letters matched a record")
	}
}

// "thumbs" finds the thumbs emoji by its name and keywords, and does not list the umbrella whose letters happen to
// contain its letters in any order.
func TestThumbsFindsThumbsEmojiNotScatteredMatches(t *testing.T) {
	results := NewCatalog().Search("thumbs", Options{})
	if len(results) == 0 || !strings.HasPrefix(results[0].Name(), "thumbs") {
		t.Fatalf("Search(thumbs) starts with %q, want a thumbs emoji", firstName(results))
	}
	for _, entry := range results {
		if strings.Contains(entry.Name(), "umbrella") {
			t.Fatalf("Search(thumbs) lists %q, a scattered-letter match", entry.Name())
		}
	}
}

func firstName(results []Entry) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].Name()
}

func TestSearchFindsEntriesByNameKeywordCharacterAndUnicode(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"name", "grinning face", "😀"},
		{"name beats keyword", "smile", "⌣"},
		{"character", "∞", "∞"},
		{"unicode value", "U+1F600", "😀"},
		{"bare hex value", "1f600", "😀"},
		{"0x value", "0x1F600", "😀"},
		{"lowercase unicode value", "u+221e", "∞"},
		{"case insensitive name", "GRINNING FACE", "😀"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := NewCatalog().Search(tt.query, Options{Limit: 3})
			if len(results) == 0 {
				t.Fatalf("Search(%q) returned nothing", tt.query)
			}
			if got := results[0].Glyph(); got != tt.want {
				t.Errorf("Search(%q)[0] = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestSearchIncludesEntriesWhoseFieldsMatchInAnyOrder(t *testing.T) {
	results := NewCatalog().Search("face grin", Options{})
	if !containsGlyph(results, "😀") {
		t.Errorf("Search(face grin) misses 😀")
	}
}

func TestSearchWithNoMatchesIsEmpty(t *testing.T) {
	if results := NewCatalog().Search("zzzzq", Options{}); len(results) != 0 {
		t.Errorf("got %d results for a nonsense query", len(results))
	}
}

func TestEmptyQueryListsCatalogInOrder(t *testing.T) {
	catalog := NewCatalog()
	all := catalog.All()
	results := catalog.Search("   ", Options{})
	if len(results) != len(all) || results[0].Glyph() != "😀" {
		t.Fatalf("empty query returned %d entries starting %q, want the full catalog", len(results), results[0].Glyph())
	}
	for index := range all {
		if all[index].ID() != results[index].ID() {
			t.Fatalf("empty query reordered entries at %d", index)
		}
	}
}

// TestSearchPutsTheEntryNamedByTheQueryFirst checks the tie rule: "fire" is also a keyword of the firefighters, which
// rank as keyword matches, and the character named fire must still lead.
func TestSearchPutsTheEntryNamedByTheQueryFirst(t *testing.T) {
	top := NewCatalog().Search("fire", Options{Limit: 1})
	if len(top) == 0 || top[0].Name() != "fire" {
		t.Fatalf("Search(fire) does not start with the fire character: %d results", len(top))
	}
}

// TestSearchOrdersTitleMatchesThenKeywordMatchesThenTheRest checks 1.x's order: entries named by the query first, then
// entries with a keyword equal to it, then the rest, and each rank in catalog order.
func TestSearchOrdersTitleMatchesThenKeywordMatchesThenTheRest(t *testing.T) {
	catalog := NewCatalog()
	idx := builtinLookups()
	for _, query := range []string{"smile", "face", "sign", "u+1f6", "heart"} {
		results := catalog.Search(query, Options{})
		needle := newSearchQuery(normalizeQuery(query))
		previousRank, previousRef := rankTitle, uint16(0)
		for _, entry := range results {
			fields, identity := recordFields(nil, entry.record(), idx, int(entry.ref)-1)
			rank := rankOf(fields[:identity], needle)
			if rank < previousRank || rank == previousRank && entry.ref <= previousRef {
				t.Fatalf("Search(%q) puts %q (rank %d) after rank %d or out of catalog order", query, entry.Name(), rank, previousRank)
			}
			previousRank, previousRef = rank, entry.ref
		}
	}
}

func TestSearchNarrowsByCategoryAndHonoursLimit(t *testing.T) {
	catalog := NewCatalog()
	limited := catalog.Search("", Options{Limit: 5})
	if len(limited) != 5 || limited[0].Glyph() != "😀" {
		t.Errorf("limit 5 returned %d entries starting %q", len(limited), limited[0].Glyph())
	}
	symbolsOnly := catalog.Search("", Options{Category: "Math"})
	for _, entry := range symbolsOnly {
		if entry.Category() != "Math" {
			t.Fatalf("Math filter returned %q in %q", entry.Glyph(), entry.Category())
		}
	}
	if unknown := catalog.Search("", Options{Category: "Nope"}); len(unknown) != len(catalog.All()) {
		t.Errorf("unknown category returned %d entries, want the full catalog", len(unknown))
	}
}

func TestSearchCategoryMatchesOnlyTheRequestedGroup(t *testing.T) {
	results := NewCatalog().Search("sign", Options{Category: "Currency"})
	if len(results) == 0 {
		t.Fatal("no Currency entries match sign")
	}
	for _, entry := range results {
		if entry.Category() != "Currency" {
			t.Errorf("%q is in %q", entry.Glyph(), entry.Category())
		}
	}
}

func TestLookupReportsMissingIDs(t *testing.T) {
	if _, ok := NewCatalog().Lookup("emoji:U+0000"); ok {
		t.Error("Lookup found an id that is not in the catalog")
	}
}

func containsGlyph(entries []Entry, glyph string) bool {
	for _, entry := range entries {
		if entry.Glyph() == glyph {
			return true
		}
	}
	return false
}
