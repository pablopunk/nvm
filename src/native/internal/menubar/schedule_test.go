package menubar

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// newTestScheduler runs apply functions inline, so a test's state is final once Tick and Wait return.
func newTestScheduler(clock *fake.Clock) *Scheduler {
	return NewScheduler(clock, func(fn func()) { fn() })
}

// settle runs due jobs until none is left to start, and waits for every run to apply its result.
func settle(s *Scheduler) {
	for range 3 {
		s.Tick()
		s.Wait()
	}
}

func TestFirstRunWaitsForTheDelayThenRepeatsOnItsCadence(t *testing.T) {
	clock := fake.NewClock()
	s := newTestScheduler(clock)
	var runs int
	s.Schedule("timer", Job{Every: time.Second, First: 2 * time.Second, Run: func(context.Context) func() {
		runs++
		return nil
	}})
	settle(s)
	if runs != 0 {
		t.Fatalf("ran before its first delay: %d", runs)
	}
	clock.Advance(2 * time.Second)
	settle(s)
	clock.Advance(time.Second)
	settle(s)
	if runs != 2 {
		t.Fatalf("runs = %d, want the first and one cadence run", runs)
	}
}

func TestApplyRunsOnTheUIDispatcher(t *testing.T) {
	clock := fake.NewClock()
	var dispatched []func()
	var mu sync.Mutex
	s := NewScheduler(clock, func(fn func()) {
		mu.Lock()
		dispatched = append(dispatched, fn)
		mu.Unlock()
	})
	var applied bool
	s.Schedule("title", Job{Run: func(context.Context) func() {
		return func() { applied = true }
	}})
	s.Now("title")
	settle(s)
	if applied {
		t.Fatal("the apply ran before the UI dispatcher was asked")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(dispatched) != 1 {
		t.Fatalf("dispatched %d applies, want 1", len(dispatched))
	}
	dispatched[0]()
	if !applied {
		t.Fatal("the dispatched apply did not run")
	}
}

func TestManualJobRunsOnlyWhenAskedAndNeverOverlaps(t *testing.T) {
	clock := fake.NewClock()
	s := newTestScheduler(clock)
	release := make(chan struct{})
	var mu sync.Mutex
	runs := 0
	s.Schedule("extension", Job{Run: func(context.Context) func() {
		mu.Lock()
		runs++
		mu.Unlock()
		<-release
		return nil
	}})
	settle(s)
	if runs != 0 {
		t.Fatalf("a manual job ran without Now: %d", runs)
	}
	s.Now("extension")
	s.Tick()
	s.Now("extension")
	s.Tick()
	close(release)
	s.Wait()
	mu.Lock()
	if runs != 1 {
		t.Fatalf("runs = %d while one was in flight, want 1", runs)
	}
	mu.Unlock()
	s.Tick()
	s.Wait()
	mu.Lock()
	defer mu.Unlock()
	if runs != 2 {
		t.Fatalf("the request made during the run did not run after it: runs = %d", runs)
	}
}

func TestCancelStopsTheJobAndKeepsNothingBehind(t *testing.T) {
	clock := fake.NewClock()
	s := newTestScheduler(clock)
	runs := 0
	s.Schedule("timer", Job{Every: time.Second, First: time.Second, Run: func(context.Context) func() {
		runs++
		return nil
	}})
	s.Cancel("timer")
	clock.Advance(time.Hour)
	settle(s)
	if runs != 0 {
		t.Fatalf("a cancelled job ran %d times", runs)
	}
	s.Now("timer")
	settle(s)
	if runs != 0 {
		t.Fatal("Now on a cancelled job ran it")
	}
}

func TestRunLoopStopsWhenItsContextEnds(t *testing.T) {
	clock := fake.NewClock()
	s := newTestScheduler(clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()
	<-done
}
