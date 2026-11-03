package app

import (
	"fmt"
	"testing"
)

// BenchmarkRootSearchKeystroke types one query into the root view and runs the same search the
// palette runs on each keystroke (Palette.search: the view's Search, then choosing the first row).
// It covers applications, commands, files, clipboard text, quicklinks and conversations. Rendering,
// the extension host and the clipboard watcher are not part of it.
func BenchmarkRootSearchKeystroke(b *testing.B) {
	p := newBenchPalette(b)
	for _, query := range benchQueries {
		b.Run("query="+query, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				p.query = query
				p.search()
			}
			benchItems = p.results
		})
	}
}

// BenchmarkClipboardHistoryFilter runs the Clipboard History view's search over the 5k-item data
// set. Text matches build full rows, which is most of the cost; images are skipped by a text query.
func BenchmarkClipboardHistoryFilter(b *testing.B) {
	p := newBenchPalette(b)
	view := p.clipboardView()
	for _, query := range benchQueries {
		b.Run("query="+query, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				benchItems = view.Search(query)
			}
		})
	}
}

// BenchmarkFileIndexSearch searches the 50k-path file index as the root view (4 results) and the
// Search Files view (40 results) do. Matching is by base name, prefix matches first.
func BenchmarkFileIndexSearch(b *testing.B) {
	p := newBenchPalette(b)
	for _, limit := range []int{4, 40} {
		for _, query := range benchQueries {
			b.Run(fmt.Sprintf("limit=%d/query=%s", limit, query), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					benchPaths = p.files.search(query, limit)
				}
			})
		}
	}
}
