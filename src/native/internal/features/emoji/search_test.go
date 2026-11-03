package emoji

import (
	"slices"
	"testing"
)

// bandOf is the band a single field reaches on its own: an ordered band, else the character-set band.
func bandOf(text, query string) int {
	if score := orderedScore(text, newSearchQuery(query)); score > 0 {
		return score
	}
	if containsEveryRune(text, query) {
		return scoreCharacterSet
	}
	return 0
}

func TestScoreBands(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		query string
		want  int
	}{
		{"exact", "grinning face", "grinning face", scoreExact},
		{"whole phrase after a word", "a smiling face", "smiling face", scoreWholePhrase},
		{"whole word inside", "big grinning face", "grinning", scoreWholePhrase},
		{"starts with but not whole", "grinning", "grinn", scoreStartsWith},
		{"contains inside a word", "xgrinningx", "grinning", scoreContains},
		{"ordered word start", "grinning face", "grinning fa", scoreOrderedWordStart},
		{"ordered word prefix", "big grinning face", "grinning fa", scoreOrderedWordPrefix},
		{"unordered word prefix", "grinning face", "face grin", scoreUnorderedWordPrefix},
		{"subsequence", "grinning face", "gnf", scoreSequential},
		{"no match", "grinning face", "zz", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bandOf(tt.text, tt.query); got != tt.want {
				t.Errorf("band of %q against %q = %d, want %d", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

// The character-set band only decides an entry when no field matches in order: an ordered match in
// any field outranks it, which keeps the fallback from changing the result of a real match.
func TestCharacterSetBandOnlyDecidesUnorderedMatches(t *testing.T) {
	if got := recordScore([]string{"grinning face"}, newSearchQuery("ecf")); got != scoreCharacterSet {
		t.Errorf("character set score = %d, want %d", got, scoreCharacterSet)
	}
	if got := recordScore([]string{"grinning face", "ecf"}, newSearchQuery("ecf")); got != scoreExact {
		t.Errorf("exact keyword score = %d, want %d", got, scoreExact)
	}
	if got := recordScore([]string{"grinning face"}, newSearchQuery("zz")); got != 0 {
		t.Errorf("no-match score = %d, want 0", got)
	}
}

func TestPhraseBoundariesIgnoreNonBMPNeighbours(t *testing.T) {
	// The TypeScript reads UTF-16 units, where a non-BMP character is never a word character.
	if got := bandOf("𝐀face", "face"); got != scoreWholePhrase {
		t.Errorf("score after a non-BMP letter = %d, want %d", got, scoreWholePhrase)
	}
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

// TestSearchPutsTheEntryNamedByTheQueryFirst checks the tie rule: "fire" is also a keyword of the firefighters,
// which score the same exact band, and the character named fire must still lead.
func TestSearchPutsTheEntryNamedByTheQueryFirst(t *testing.T) {
	top := NewCatalog().Search("fire", Options{Limit: 1})
	if len(top) == 0 || top[0].Name() != "fire" {
		t.Fatalf("Search(fire) does not start with the fire character: %d results", len(top))
	}
}

func TestSearchResultsAreOrderedByBestScore(t *testing.T) {
	catalog := NewCatalog()
	for _, query := range []string{"smile", "face", "sign", "u+1f6", "heart"} {
		results := catalog.Search(query, Options{})
		needle := newSearchQuery(normalizeQuery(query))
		previous := scoreExact + 1
		idx := builtinLookups()
		for _, entry := range results {
			fields := recordFields(nil, entry.record(), idx, int(entry.ref)-1)
			score := recordScore(fields, needle)
			if score > previous {
				t.Fatalf("Search(%q) puts score %d after %d", query, score, previous)
			}
			previous = score
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

type fakeRecents []string

func (f fakeRecents) RecentIDs() []string { return f }

func TestRecentsResolvesIDsNewestFirstWithoutDuplicates(t *testing.T) {
	catalog := NewCatalog()
	source := fakeRecents{"emoji:U+1F600", "bogus:id", "emoji:U+1F600", "symbol:U+2713"}
	recents := catalog.Recents(source, 0)
	if len(recents) != 2 || recents[0].Glyph() != "😀" || recents[1].Glyph() != "✓" {
		t.Fatalf("Recents = %v, want [😀 ✓]", glyphsOf(recents))
	}
	if limited := catalog.Recents(source, 1); len(limited) != 1 || limited[0].Glyph() != "😀" {
		t.Errorf("limited Recents = %v, want [😀]", glyphsOf(limited))
	}
	if nilSource := catalog.Recents(nil, 5); nilSource != nil {
		t.Errorf("nil source returned %v", glyphsOf(nilSource))
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

func glyphsOf(entries []Entry) []string {
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Glyph())
	}
	return slices.Clip(out)
}
