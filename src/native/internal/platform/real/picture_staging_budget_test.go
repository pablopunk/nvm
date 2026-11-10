package real

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPictureStagingReservationsBoundConcurrentCopiesAndRelease(t *testing.T) {
	budget := newPictureStagingBudget(10)
	available := func() (int64, error) { return pictureStagingReserve + 100, nil }
	releaseFirst, err := budget.reserve(context.Background(), 8, available)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		release func()
		err     error
	}
	next := make(chan result, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { release, err := budget.reserve(ctx, 4, available); next <- result{release, err} }()
	budget.mu.Lock()
	used := budget.used
	budget.mu.Unlock()
	if used != 8 {
		t.Fatalf("concurrent copies exceeded reservation: %d", used)
	}
	releaseFirst()
	second := <-next
	if second.err != nil {
		t.Fatal(second.err)
	}
	// Cleanup can be called both explicitly and by a deferred owner without releasing twice.
	releaseFirst()
	second.release()
	second.release()
	if budget.used != 0 {
		t.Fatalf("cleanup retained %d bytes", budget.used)
	}
}

func TestPictureStagingRespectsFreeSpaceAndCancellation(t *testing.T) {
	budget := newPictureStagingBudget(10)
	if _, err := budget.reserve(context.Background(), 4, func() (int64, error) { return pictureStagingReserve + 3, nil }); !errors.Is(err, errPictureStagingSpace) {
		t.Fatalf("low-space reservation = %v", err)
	}
	available := func() (int64, error) { return pictureStagingReserve + 100, nil }
	release, err := budget.reserve(context.Background(), 10, available)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := budget.reserve(ctx, 1, available); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled queued reservation = %v", err)
	}
	if _, err := budget.reserve(context.Background(), 11, available); !errors.Is(err, errPictureStagingSpace) {
		t.Fatalf("oversized reservation = %v", err)
	}
}
