package manager

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type heldSearchBuiltin struct {
	started chan struct{}
	release chan struct{}
}

func (heldSearchBuiltin) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.held-search", Title: "Held"}}
}
func (b heldSearchBuiltin) SearchItems(api.Context, string) ([]api.Item, error) {
	close(b.started)
	<-b.release
	return []api.Item{{ID: "late", Title: "Late"}}, nil
}

func TestSearchTargetsAnnouncedBeforeCancellationAndNoLateDelivery(t *testing.T) {
	held := heldSearchBuiltin{started: make(chan struct{}), release: make(chan struct{})}
	h := newHarness(t, withBuiltIns(held))
	ctx, cancel := context.WithCancel(context.Background())
	announced := make(chan []string, 1)
	results := make(chan SearchResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.m.SearchEachWithTargets(ctx, "q", func(ids []string) { announced <- ids }, func(result SearchResult) { results <- result })
	}()
	ids := <-announced
	if len(ids) != 1 || ids[0] != "nevermind.held-search" {
		t.Fatalf("targets = %v", ids)
	}
	<-held.started
	cancel()
	<-done
	close(held.release)
	select {
	case result := <-results:
		t.Fatalf("cancelled search published %+v", result)
	default:
	}
}
