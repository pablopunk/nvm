package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// pendingProvider reports a call in flight at once, then answers with one row when release is closed.
type pendingProvider struct {
	release chan struct{}
}

func (p pendingProvider) ID() string                  { return "pending" }
func (p pendingProvider) Root(context.Context) []Item { return nil }

func (p pendingProvider) Search(ctx context.Context, _ string, emit func(Batch)) {
	emit(Batch{Pending: true})
	select {
	case <-p.release:
	case <-ctx.Done():
	}
	emit(Batch{Items: []Item{{ID: "late", Title: "Late"}}, Final: true})
}

// TestPendingCallIsSnapshotUntilTheProviderFinishes checks a provider's in-flight call shows as Pending on the
// snapshot, and that the snapshot is complete and not pending once its Final batch arrives.
func TestPendingCallIsSnapshotUntilTheProviderFinishes(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot})
	release := make(chan struct{})
	c.Query("ca", []Provider{pendingProvider{release: release}})
	passFirstSnapshot(t, c, clock)

	first := sink.next(t)
	if !first.Pending || first.Settled {
		t.Fatalf("first snapshot = %+v, want pending and not settled while the call runs", first)
	}
	close(release)
	snaps := sink.untilSettled(t, first.Gen)
	last := snaps[len(snaps)-1]
	if last.Pending || !last.Settled || titles(last.Items)[0] != "Late" {
		t.Fatalf("last snapshot = %+v, want the late row, settled and not pending", last)
	}
}

// TestSynchronousPendingCallIsReportedWhileUnfinished checks a synchronous query whose provider never finishes
// reports the call as pending, which is what the palette shows while the second batch is in flight.
func TestSynchronousPendingCallIsReportedWhileUnfinished(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true})
	snap, ready := c.Query("ca", []Provider{stalledProvider{}})
	if !ready || !snap.Pending || snap.Settled {
		t.Fatalf("snapshot = %+v, want pending and not settled", snap)
	}
}

// stalledProvider reports a call in flight and never finishes it.
type stalledProvider struct{}

func (stalledProvider) ID() string                  { return "stalled" }
func (stalledProvider) Root(context.Context) []Item { return nil }

func (stalledProvider) Search(_ context.Context, _ string, emit func(Batch)) {
	emit(Batch{Pending: true})
}
