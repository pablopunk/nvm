package emoji

import "testing"

// benchmarkQueries cover the kinds of input the picker sees: a name, a keyword, a character, a Unicode
// value, a multi-word phrase, a query that matches nothing, and the empty query.
var benchmarkQueries = []struct {
	name  string
	query string
}{
	{"name", "grinning face"},
	{"keyword", "smile"},
	{"common-letter", "a"},
	{"character", "∞"},
	{"unicode", "U+1F600"},
	{"multi-word", "face grin"},
	{"no-match", "zzzzq"},
	{"empty", ""},
}

func BenchmarkSearchFullCatalog(b *testing.B) {
	catalog := NewCatalog()
	catalog.Search("warm up", Options{})
	for _, bench := range benchmarkQueries {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				catalog.Search(bench.query, Options{})
			}
		})
	}
}

func BenchmarkSearchCategory(b *testing.B) {
	catalog := NewCatalog()
	b.ReportAllocs()
	for range b.N {
		catalog.Search("sign", Options{Category: "Currency"})
	}
}

// BenchmarkBuildLookups measures the heap cost of the derived indexes (labels, id map, rune map).
// The static tables live in the binary and do not appear here.
func BenchmarkBuildLookups(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = buildLookups()
	}
}
