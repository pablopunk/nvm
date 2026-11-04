package manager

import (
	"context"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestInvalidateRootCallReachesTheHostHook runs the acme.fallback fixture in its child process. Its bump action asks the
// host to refresh the root, and the host hook has the call before the action returns.
func TestInvalidateRootCallReachesTheHostHook(t *testing.T) {
	var mu sync.Mutex
	var refreshed []string
	h := newHarness(t, func(o *Options) {
		o.RootRefresh = func(id string) {
			mu.Lock()
			defer mu.Unlock()
			refreshed = append(refreshed, id)
		}
	})
	if err := h.m.Install("acme.fallback", fixtureContents(t, "acme.fallback"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.fallback")
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	_, failure, err := h.m.Run(ctx, "acme.fallback", api.Invocation{Ref: api.ActionRef{ID: "bump"}})
	if err != nil || failure != nil {
		t.Fatalf("Run bump: err %v failure %+v", err, failure)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(refreshed) != 1 || refreshed[0] != "acme.fallback" {
		t.Fatalf("root refresh calls = %v; want one for acme.fallback", refreshed)
	}
}

// TestRootsOfAsksOnlyTheMarkedExtensions checks the scoped root query that an InvalidateRoot refresh uses: the extension
// not marked is never called, so its rows are not asked again.
func TestRootsOfAsksOnlyTheMarkedExtensions(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"acme.fallback", "acme.good"} {
		if err := h.m.Install(id, fixtureContents(t, id), true); err != nil {
			t.Fatalf("Install %s: %v", id, err)
		}
		h.waitRunning(id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	results := h.m.RootsOf(ctx, map[string]bool{"acme.good": true})
	if len(results) != 1 || results[0].ExtensionID != "acme.good" {
		t.Fatalf("RootsOf(acme.good) answered %+v; want only acme.good", results)
	}
}
