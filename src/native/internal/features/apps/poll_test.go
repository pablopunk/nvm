package apps

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The running-status poll runs only while the palette is visible: started on show, stopped on hide, on the fake clock.
func TestPollRunsOnlyUntilStopped(t *testing.T) {
	clock := fake.NewClock()
	ticks := make(chan struct{})
	stop := Poll(clock, RunningTTL, func() { ticks <- struct{}{} })

	clock.AwaitTimers(1)
	for range 3 {
		clock.Advance(RunningTTL)
		<-ticks
	}
	stop()
	if clock.PendingTimers() != 0 {
		t.Fatalf("pending timers after stop = %d, want 0", clock.PendingTimers())
	}
	clock.Advance(10 * time.Minute)
	select {
	case <-ticks:
		t.Fatal("poll ran after stop")
	default:
	}
	stop()
}

// Stop returns once no call is running, so the caller can rely on the poll being quiet after it.
func TestPollStopWaitsForRunningCall(t *testing.T) {
	clock := fake.NewClock()
	entered := make(chan struct{})
	release := make(chan struct{})
	stop := Poll(clock, time.Second, func() {
		entered <- struct{}{}
		<-release
	})
	clock.AwaitTimers(1)
	clock.Advance(time.Second)
	<-entered
	go func() { close(release) }()
	stop()
	if clock.PendingTimers() != 0 {
		t.Fatal("ticker still registered after stop")
	}
}
