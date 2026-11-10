package palette

import (
	"context"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestDeferredEnterWaitsForMatchingQuery(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	releaseNew := make(chan struct{})
	provider := search.NewRowsProvider("held", func(ctx context.Context, q string) []search.Item {
		if q == "new" {
			select {
			case <-releaseNew:
			case <-ctx.Done():
				return nil
			}
		}
		query := q
		return []search.Item{{ID: "row-" + query, Kind: "Test", Title: query, Actions: []search.Action{{Title: "Run " + query, Dismiss: search.DismissNever, Run: func(search.Host) {
			mu.Lock()
			ran = append(ran, query)
			mu.Unlock()
		}}}}}
	})
	root := &View{Title: "Root", Providers: func() []search.Provider { return []search.Provider{provider} }}
	h := newHarness(t, root, false)
	h.p.SetQuery("old")
	h.awaitRows(func(titles []string) bool { return len(titles) == 1 && titles[0] == "old" })
	h.key(Mod, ui.KeyA)
	h.typeText("new")
	if h.p.Query() != "new" || h.chosenTitle() != "old" {
		t.Fatalf("before Enter: query=%q selected=%q, want new query with old row still shown", h.p.Query(), h.chosenTitle())
	}
	h.key(0, ui.KeyEnter)
	mu.Lock()
	if len(ran) != 0 {
		mu.Unlock()
		t.Fatalf("Enter on pending query ran %v", ran)
	}
	mu.Unlock()
	if h.p.defaultEnter == nil || h.p.defaultEnter.query != "new" {
		t.Fatalf("pending Enter intent = %#v, want query new", h.p.defaultEnter)
	}
	close(releaseNew)
	h.awaitRows(func(titles []string) bool { return len(titles) == 1 && titles[0] == "new" && h.p.Settled() })
	mu.Lock()
	if len(ran) != 1 || ran[0] != "new" {
		mu.Unlock()
		t.Fatalf("after matching rows settled, ran %v; want [new]", ran)
	}
	mu.Unlock()
	h.frame()
	h.frame()
	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 1 {
		t.Fatalf("extra frames replayed default action: %v", ran)
	}
}
