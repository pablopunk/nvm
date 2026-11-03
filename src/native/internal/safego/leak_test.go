package safego

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestCallsLeaveNoGoroutinesBehind exercises every entry point with timeouts, restarts and cancellation,
// then requires the goroutine count to return to the baseline. The group case depends on the sibling
// cancellation: if Group stopped cancelling, waitWithWatchdog fails the test instead of hanging. Removing
// the cancel in Group.fail was checked this way: both group tests fail with the watchdog message.
func TestCallsLeaveNoGoroutinesBehind(t *testing.T) {
	clock := fake.NewClock()
	rep := newRecordingReporter("")
	baseline := runtime.NumGoroutine()

	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	if err := Run("leak.run.ok", func(context.Context) error { return nil },
		WithClock(clock), WithReporter(rep), WithTimeout(time.Second)); err != nil {
		t.Fatalf("Run ok = %v", err)
	}

	Go("leak.go.timeout", func(ctx context.Context) { <-ctx.Done() },
		WithClock(clock), WithReporter(rep), WithTimeout(time.Second), WithContext(parent))
	clock.AwaitTimers(1)
	clock.Advance(time.Second)

	Go("leak.go.backoff", func(context.Context) { panic("loop") },
		WithClock(clock), WithReporter(rep), WithBackoff(10, time.Minute), WithContext(parent))
	clock.AwaitTimers(1)
	cancelParent()

	g := NewGroup(WithReporter(rep))
	g.Go("leak.group.fail", func(context.Context) error { return errors.New("stop") })
	g.Go("leak.group.wait", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := waitWithWatchdog(t, g); err == nil {
		t.Fatal("group Wait = nil, want the first failure")
	}

	waitForGoroutines(t, baseline)
}

func TestWatchCountsLiveGoroutinesByName(t *testing.T) {
	const name = "test.watch.live"
	release := make(chan struct{})
	started := make(chan struct{})
	Go(name, func(context.Context) {
		close(started)
		<-release
	}, WithReporter(newRecordingReporter("")))

	<-started
	if got := Watching().Live(name); got != 1 {
		t.Fatalf("Live(%q) = %d while running, want 1", name, got)
	}
	close(release)
	waitForLive(t, name, 0)
}
