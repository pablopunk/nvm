package real

import (
	"context"
	"errors"
	"sync"
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
	waiter := &observedStagingWait{Context: ctx, waiting: make(chan struct{})}
	go func() { release, err := budget.reserve(waiter, 4, available); next <- result{release, err} }()
	select {
	case <-waiter.waiting:
	case early := <-next:
		if early.release != nil {
			early.release()
		}
		t.Fatalf("second copy did not wait: %v", early.err)
	case <-ctx.Done():
		t.Fatal("second copy never reached the capacity wait")
	}
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

type observedStagingWait struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (w *observedStagingWait) Done() <-chan struct{} {
	w.once.Do(func() { close(w.waiting) })
	return w.Context.Done()
}

func TestQueuedPictureStagingRequestCanBeCancelled(t *testing.T) {
	budget := newPictureStagingBudget(10)
	available := func() (int64, error) { return pictureStagingReserve + 100, nil }
	release, err := budget.reserve(context.Background(), 10, available)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	waiter := &observedStagingWait{Context: ctx, waiting: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		release, err := budget.reserve(waiter, 1, available)
		if release != nil {
			release()
		}
		result <- err
	}()
	select {
	case <-waiter.waiting:
	case err := <-result:
		t.Fatalf("request did not queue: %v", err)
	case <-ctx.Done():
		t.Fatal("request never reached the wait")
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued cancellation = %v", err)
	}
}

func TestFailedPictureCleanupIsRetriedBeforeAnotherReservation(t *testing.T) {
	budget := newPictureStagingBudget(10)
	available := func() (int64, error) { return pictureStagingReserve + 100, nil }
	release, err := budget.reserve(context.Background(), 10, available)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	cleanup := budget.cleanup("staged-source", func(string) error {
		calls++
		if calls == 1 {
			return errors.New("temporary removal failure")
		}
		return nil
	}, release)
	cleanup()
	if budget.used != 10 || len(budget.pending) != 1 {
		t.Fatal("failed cleanup lost the file or its reservation")
	}
	// The staging caller may already have returned an error and discarded its cleanup function.
	next, err := budget.reserve(context.Background(), 10, available)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(budget.pending) != 0 {
		t.Fatal("next request did not retry failed staging cleanup")
	}
	cleanup()
	if calls != 2 || budget.used != 10 {
		t.Fatal("repeated successful cleanup removed or released twice")
	}
	next()
	if budget.used != 0 {
		t.Fatal("reservation was stranded")
	}
}
