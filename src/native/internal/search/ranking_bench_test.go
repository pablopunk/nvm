package search

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// benchNameSizes are the name counts the ranking benchmark scores: the 10k, 50k and 100k ranges of
// PLAN.md's J29 scale row.
var benchNameSizes = []int{10_000, 50_000, 100_000}

// benchKeystrokes are the queries the ranking benchmark types, one per keystroke length class.
var benchKeystrokes = []string{"cl", "clip", "show the time"}

// benchNameCorpora caches the lowercased base names of the fixture tree, by count. The benchmark
// framework runs each benchmark function several times, and generating 100k paths once is enough.
var benchNameCorpora = map[int][]string{}

// benchSink keeps scores alive so the compiler cannot discard the work a benchmark measures.
var benchSink int

// benchNames returns n lowercased file base names from the fixture tree. Scoring uses the lowercased
// names, as the palette does before it ranks them.
func benchNames(n int) []string {
	if names, ok := benchNameCorpora[n]; ok {
		return names
	}
	paths := fixtures.Files(n)
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = strings.ToLower(filepath.Base(path))
	}
	benchNameCorpora[n] = names
	return names
}

// BenchmarkScoreFuzzy scores every name of a corpus against one keystroke query. ns/op is one full
// pass; ns/name divides it by the corpus size.
func BenchmarkScoreFuzzy(b *testing.B) {
	for _, size := range benchNameSizes {
		names := benchNames(size)
		for _, query := range benchKeystrokes {
			b.Run(fmt.Sprintf("names=%d/query=%s", size, query), func(b *testing.B) {
				b.ReportAllocs()
				sink := 0
				for range b.N {
					for _, name := range names {
						sink += ScoreFuzzy(name, query)
					}
				}
				benchSink = sink
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(names)), "ns/name")
			})
		}
	}
}
