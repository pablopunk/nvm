package clipboard

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// newBenchHistory returns a history of n text entries of about one line each, the J29 shape for clipboard
// history. The entries are loaded from a saved snapshot, as the app does at start, so the setup does not copy
// the history once per entry. Expiry is off so the entries stay.
func newBenchHistory(tb testing.TB, n int) *History {
	tb.Helper()
	clock := fake.NewClock()
	entries := make([]Entry, 0, n)
	for i := range n {
		text := fmt.Sprintf("entry %d: clipboard snippet with lorem ipsum dolor sit amet, number %d", i, i*7919%10007)
		entries = append(entries, Entry{ID: TextID(text), Kind: KindText, Text: text, CreatedAt: clock.Now()})
	}
	persist := NewMemoryPersistence()
	persist.SetStored(Snapshot{Version: SnapshotVersion, Entries: entries})
	h, err := Open(persist, clock, Retention{
		MaxEntries:     n + 1,
		MaxAge:         -1,
		MaxImageBytes:  DefaultMaxImageBytes,
		MaxMemoryBytes: 1 << 30,
	})
	if err != nil {
		tb.Fatalf("Open: %v", err)
	}
	return h
}

// BenchmarkSearchFiveThousand measures the J29 search: 5,000 entries, a query that matches many rows, one that
// matches a few, and one that matches none (a full scan).
func BenchmarkSearchFiveThousand(b *testing.B) {
	h := newBenchHistory(b, 5000)
	queries := []struct{ name, text string }{
		{"many-matches", "lorem"},
		{"few-matches", "number 4242"},
		{"no-match", "zzz-absent"},
	}
	for _, q := range queries {
		b.Run(q.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = h.Search(q.text)
			}
		})
	}
}

// TestMemoryEstimateForFiveThousandEntries reports the heap held by 5,000 entries and checks it against a
// budget far below the 100 MB idle budget (B4). The number is logged for the WP report.
func TestMemoryEstimateForFiveThousandEntries(t *testing.T) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	h := newBenchHistory(t, 5000)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	t.Logf("5000 text entries: heap %d bytes (%.0f bytes per entry), stats %+v", heap, float64(heap)/5000, h.Stats())
	const budget = 16 << 20
	if heap > budget {
		t.Fatalf("5000 entries hold %d bytes, over the %d budget", heap, int64(budget))
	}
	runtime.KeepAlive(h)
}
