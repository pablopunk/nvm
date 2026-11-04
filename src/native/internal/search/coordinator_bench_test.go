package search

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// bestRows scores every name against q and keeps the best limit of them, the way a file source answers a
// keystroke: one full scan of its index, keeping only the rows it will show.
func bestRows(names []string, q string, limit int) []Item {
	var top []Item
	for _, name := range names {
		s := ScoreFuzzy(name, q)
		if s == 0 || (len(top) == limit && s <= top[len(top)-1].Score) {
			continue
		}
		it := Item{Kind: KindFiles, Title: name, Score: s}
		i := len(top)
		for i > 0 && top[i-1].Score < s {
			i--
		}
		top = append(top, Item{})
		copy(top[i+1:], top[i:])
		top[i] = it
		if len(top) > limit {
			top = top[:limit]
		}
	}
	return top
}

// mergeOnlyRows is a source that answers from memory: forty rows, scored, so a benchmark of it measures the
// coordinator's own work (goroutines, budgets, merge, cache) without a scan.
func mergeOnlyRows(id string) Provider {
	rows := make([]Item, 40)
	for i := range rows {
		rows[i] = Item{Kind: KindFiles, Title: fmt.Sprintf("%s row %d", id, i), Score: 40 - i}
	}
	return fixedRows(id, rows...)
}

// BenchmarkCoordinatorKeystroke measures one keystroke through the coordinator. The scan case runs a source over
// the 50k-name fixture index (PLAN.md J29) and a commands source; the merge case runs two in-memory sources of
// forty rows each. Synchronous is the work alone; async adds the goroutines, the budget watch and the snapshot
// hand-off, timed to the settled snapshot with the real clock. ns/op is the cost of one keystroke.
func BenchmarkCoordinatorKeystroke(b *testing.B) {
	names := benchNames(50_000)
	scan := []Provider{
		NewRowsProvider("files", func(_ context.Context, q string) []Item { return bestRows(names, q, DefaultLimit) }),
		fixedRows("commands", Item{Kind: KindCommands, Title: "Clipboard History", Score: 90}),
	}
	merge := []Provider{mergeOnlyRows("apps"), mergeOnlyRows("files")}
	cases := []struct {
		name      string
		providers []Provider
	}{{"scan50k", scan}, {"merge", merge}}
	for _, c := range cases {
		for _, query := range benchKeystrokes {
			b.Run(fmt.Sprintf("sync/%s/query=%s", c.name, query), func(b *testing.B) {
				co := NewCoordinator(Options{Clock: real.NewPorts().Clock, Synchronous: true})
				b.ReportAllocs()
				for range b.N {
					snap, _ := co.Query(query, c.providers)
					benchSink += len(snap.Items)
				}
			})
			b.Run(fmt.Sprintf("async/%s/query=%s", c.name, query), func(b *testing.B) {
				settled := make(chan Snapshot, 64)
				co := NewCoordinator(Options{Clock: real.NewPorts().Clock, OnSnapshot: func(s Snapshot) {
					if s.Settled {
						settled <- s
					}
				}})
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					co.Query(query, c.providers)
					select {
					case snap := <-settled:
						benchSink += len(snap.Items)
					case <-time.After(watchdog):
						b.Fatal("the query never settled")
					}
				}
			})
		}
	}
}
