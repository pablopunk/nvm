package palette

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// withClock makes the harness use the test's clock, so the test can see and move the time providers wait on.
func withClock(clock *fake.Clock) func(*Options) {
	return func(o *Options) { o.Clock = clock }
}

// TestBlockedProviderDoesNotDelayRowsOrTyping runs a root with a provider that waits two seconds of fake time.
// The rows of the other provider reach the screen, and the keys are taken, before the clock moves.
func TestBlockedProviderDoesNotDelayRowsOrTyping(t *testing.T) {
	clock := fake.NewClock()
	blocked := search.NewRowsProvider("blocked", func(context.Context, string) []search.Item {
		<-clock.After(2 * time.Second)
		return []search.Item{{Kind: "Test", Title: "late"}}
	})
	quick := fixedProvider("quick", search.Item{Kind: "Test", Title: "Quick", Score: 1})
	root := &View{Title: "Root", Providers: func() []search.Provider { return []search.Provider{blocked, quick} }}
	h := newHarness(t, root, false, withClock(clock))

	h.typeText("q")
	clock.AwaitTimers(1)
	h.awaitRows(func(titles []string) bool { return slices.Contains(titles, "Quick") })
	if h.p.Query() != "q" {
		t.Fatalf("typing waited for the blocked provider: query = %q", h.p.Query())
	}
	if h.p.Settled() {
		t.Fatal("the query settled while a provider was still blocked")
	}

	// The hard budget (100 ms) cuts the provider off long before its two seconds: it is counted as slow and the
	// query settles without it. Move the clock past the budget first, then past the provider's own wait, so its
	// late rows arrive after the cut-off (advancing both at once would let the two race).
	clock.Advance(150 * time.Millisecond)
	h.awaitRows(func([]string) bool { return h.p.Settled() })
	clock.Advance(2 * time.Second)
	if got := h.titles(); slices.Contains(got, "late") || !slices.Contains(got, "Quick") {
		t.Fatalf("after the cut-off the rows should be the quick ones only, got %v", got)
	}
	slow := 0
	for _, s := range h.p.cur().coord.Stats() {
		if s.ID == "blocked" {
			slow = s.Slow
		}
	}
	if slow != 1 {
		t.Fatalf("the blocked provider should be counted as slow once, got %d", slow)
	}
}

// TestCachedRowsShowAtOnceAndRefreshInPlace checks that a query typed again shows the rows it had before the
// providers run, and that the fresh rows replace them without a new query.
func TestCachedRowsShowAtOnceAndRefreshInPlace(t *testing.T) {
	var mu sync.Mutex
	version := 1
	provider := search.NewRowsProvider("versioned", func(_ context.Context, q string) []search.Item {
		mu.Lock()
		defer mu.Unlock()
		return []search.Item{{Kind: "Test", Title: fmt.Sprintf("v%d %s", version, q)}}
	})
	root := &View{Title: "Root", Providers: func() []search.Provider { return []search.Provider{provider} }}
	h := newHarness(t, root, false)

	h.p.SetQuery("a")
	h.awaitRows(func(titles []string) bool { return slices.Equal(titles, []string{"v1 a"}) })
	h.p.SetQuery("b")
	h.awaitRows(func(titles []string) bool { return slices.Equal(titles, []string{"v1 b"}) })

	mu.Lock()
	version = 2
	mu.Unlock()
	h.p.SetQuery("a")
	if got := h.titles(); !slices.Equal(got, []string{"v1 a"}) {
		t.Fatalf("a repeated query should show its remembered rows at once, got %v", got)
	}
	h.awaitRows(func(titles []string) bool { return slices.Equal(titles, []string{"v2 a"}) })
}

// TestEmptyStateWaitsForTheProviders checks that a query whose providers are still answering shows no empty
// state: the list is blank until the answer is in.
func TestEmptyStateWaitsForTheProviders(t *testing.T) {
	clock := fake.NewClock()
	slow := search.NewRowsProvider("slow", func(context.Context, string) []search.Item {
		<-clock.After(time.Second)
		return nil
	})
	root := &View{Title: "Root", EmptyTitle: "Nothing here", Providers: func() []search.Provider { return []search.Provider{slow} }}
	h := newHarness(t, root, false, withClock(clock))
	h.typeText("zz")
	clock.AwaitTimers(1)
	if h.p.Settled() {
		t.Fatal("a pending query must not report itself settled")
	}
	clock.Advance(time.Second)
	h.awaitRows(func([]string) bool { return h.p.Settled() })
}

// TestSelectionIsAnnouncedAndEveryRowIsLabeled checks what assistive technology hears: the title of the row the
// cursor is on, announced again when the cursor moves, and a label on every row of the list.
func TestSelectionIsAnnouncedAndEveryRowIsLabeled(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha", "Beta", "Gamma"), true)
	if got := h.tt.Announcements(); !slices.Contains(got, "Alpha") {
		t.Fatalf("the first row should be announced on show, got %v", got)
	}

	h.key(0, ui.KeyDown)
	if got := h.tt.Announcements(); !slices.Contains(got, "Beta") {
		t.Fatalf("moving down should announce the new row, got %v", got)
	}

	for _, title := range []string{"Alpha", "Beta", "Gamma"} {
		if _, ok := h.tt.Find(title); !ok {
			t.Fatalf("row %q has no label for assistive technology", title)
		}
	}
}
