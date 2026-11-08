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

func TestInvalidateViewCarriesOriginFromSupervisedInvocation(t *testing.T) {
	var mu sync.Mutex
	var gotID string
	var gotIdentity any
	h := newHarness(t, func(o *Options) {
		o.ViewsInvalidate = func(id string, identity any) {
			mu.Lock()
			defer mu.Unlock()
			gotID, gotIdentity = id, identity
		}
	})
	if err := h.m.Install("acme.fallback", fixtureContents(t, "acme.fallback"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.fallback")
	identity := h.m.SnapshotIdentity("acme.fallback")
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	_, failure, err := h.m.Run(ctx, "acme.fallback", api.Invocation{Ref: api.ActionRef{ID: "invalidate-view"}})
	if err != nil || failure != nil {
		t.Fatalf("run: err=%v failure=%v", err, failure)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotID != "acme.fallback" || gotIdentity != identity {
		t.Fatalf("got id=%q identity=%v want=%v", gotID, gotIdentity, identity)
	}
}

func TestSupervisedHeldInvalidationRetainsInvocationOrigin(t *testing.T) {
	entered := make(chan any, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	h := newHarness(t, func(o *Options) {
		o.ViewInvalidate = func(_ string, identity any) {
			entered <- identity
			<-release
		}
	})
	const id = "acme.fallback"
	if err := h.m.Install(id, fixtureContents(t, id), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	identity := h.m.SnapshotIdentity(id)
	type outcome struct {
		result  api.Result
		failure *api.Failure
		err     error
	}
	results := make(chan outcome, 1)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	go func() {
		result, failure, err := h.m.Run(ctx, id, api.Invocation{Ref: api.ActionRef{ID: "held-invalidation"}, RequireIdentity: true, ExpectedIdentity: identity})
		results <- outcome{result, failure, err}
	}()
	select {
	case got := <-entered:
		if got != identity {
			t.Fatalf("service origin=%v", got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	unblock()
	select {
	case got := <-results:
		if got.err != nil || got.failure != nil || !got.result.DataChanged || !got.result.OriginBound || got.result.OriginIdentity != identity {
			t.Fatalf("result=%+v failure=%v err=%v", got.result, got.failure, got.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
