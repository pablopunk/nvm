package services

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// rootRefreshHandler returns the services of one extension whose RootRefresh hook counts its calls.
func rootRefreshHandler(t *testing.T, r rig, calls *int) *Handler {
	t.Helper()
	ports := real.NewPorts()
	handler := New(Deps{
		Files:       ports.FS,
		Clock:       r.clock,
		DataRoot:    r.root,
		Keychain:    r.keychain,
		Network:     r.network,
		Clipboard:   r.clipboard,
		Logs:        r.logs,
		RootRefresh: func(string) { *calls++ },
	}, manifestFor("acme.fallback"))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// TestInvalidateRootIsBurstLimitedAndNeverFails checks 1.x's rule of five refreshes in a burst: a call over the limit
// is ignored without an error, so a RootItems that refreshes itself cannot loop, and the bucket refills with the clock.
func TestInvalidateRootIsBurstLimitedAndNeverFails(t *testing.T) {
	r := newRig(t)
	calls := 0
	h := rootRefreshHandler(t, r, &calls)
	for range 7 {
		if reply := serve(t, h, protocol.ServiceViews, protocol.OpInvalidateRoot, nil); reply.Failure != nil {
			t.Fatalf("invalidateRoot failed: %+v", reply.Failure)
		}
	}
	if calls != rootRefreshBurst {
		t.Fatalf("calls = %d, want the burst of %d", calls, rootRefreshBurst)
	}
	r.clock.Advance(500 * time.Millisecond)
	serve(t, h, protocol.ServiceViews, protocol.OpInvalidateRoot, nil)
	if calls != rootRefreshBurst+1 {
		t.Fatalf("after 500 ms calls = %d, want one refill", calls)
	}
}
