package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartupSyncWaitsForExtensionsAndTheirListingsThenRequestsOnce(t *testing.T) {
	settled := make(chan struct{})
	var known, requests atomic.Int32
	done := make(chan struct{})
	go func() {
		awaitStartupSync(context.Background(), startupSync{
			settled: settled, poll: time.Millisecond, limit: time.Minute,
			listingsKnown: func() bool { return known.Load() == 1 },
			request:       func() { requests.Add(1) },
		})
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	if requests.Load() != 0 {
		t.Fatal("synced before the extensions loaded")
	}
	close(settled)
	time.Sleep(20 * time.Millisecond)
	if requests.Load() != 0 {
		t.Fatal("synced before the extensions listed their commands")
	}
	known.Store(1)
	<-done
	if requests.Load() != 1 {
		t.Fatalf("requests %d", requests.Load())
	}
}

func TestStartupSyncDoesNotWaitForeverForListings(t *testing.T) {
	settled := make(chan struct{})
	close(settled)
	var requests int
	awaitStartupSync(context.Background(), startupSync{
		settled: settled, poll: time.Millisecond, limit: 20 * time.Millisecond,
		listingsKnown: func() bool { return false }, request: func() { requests++ },
	})
	if requests != 1 {
		t.Fatalf("requests %d", requests)
	}
}

func TestStartupSyncStopsWhenTheAppEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requests := 0
	awaitStartupSync(ctx, startupSync{settled: make(chan struct{}), request: func() { requests++ }})
	if requests != 0 {
		t.Fatal("requested after the app ended")
	}
}
