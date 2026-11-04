package fake

import (
	"testing"
	"time"
)

func TestClockStartsAtFixtureEpochAndMovesOnlyWhenAdvanced(t *testing.T) {
	clock := NewClock()
	if got := clock.Now(); !got.Equal(epoch) {
		t.Fatalf("Now() = %v, want %v", got, epoch)
	}
	clock.Advance(90 * time.Second)
	if got := clock.Now(); !got.Equal(epoch.Add(90 * time.Second)) {
		t.Fatalf("after Advance, Now() = %v", got)
	}
}

func TestAfterFiresInDeadlineOrder(t *testing.T) {
	clock := NewClock()
	slow := clock.After(2 * time.Second)
	fast := clock.After(time.Second)

	clock.Advance(time.Second)
	assertTimeReceived(t, fast, epoch.Add(time.Second))
	assertNothingReceived(t, slow)

	clock.Advance(time.Second)
	assertTimeReceived(t, slow, epoch.Add(2*time.Second))
}

func TestAfterNonPositiveDurationFiresImmediately(t *testing.T) {
	clock := NewClock()
	assertTimeReceived(t, clock.After(0), epoch)
}

func TestTickerDropsTicksNobodyReads(t *testing.T) {
	clock := NewClock()
	ticker := clock.NewTicker(time.Second)
	defer ticker.Stop()

	clock.Advance(5 * time.Second)
	assertTimeReceived(t, ticker.C(), epoch.Add(time.Second))
	assertNothingReceived(t, ticker.C())

	clock.Advance(time.Second)
	assertTimeReceived(t, ticker.C(), epoch.Add(6*time.Second))
}

func TestStoppedTickerNoLongerFires(t *testing.T) {
	clock := NewClock()
	ticker := clock.NewTicker(time.Second)
	ticker.Stop()
	clock.Advance(10 * time.Second)
	assertNothingReceived(t, ticker.C())
	if pending := clock.PendingTimers(); pending != 0 {
		t.Fatalf("PendingTimers() = %d after Stop, want 0", pending)
	}
}

func TestNewTickerPanicsOnNonPositivePeriod(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewTicker(0) did not panic")
		}
	}()
	NewClock().NewTicker(0)
}

func TestSleepReturnsOnlyAfterAdvance(t *testing.T) {
	clock := NewClock()
	done := make(chan struct{})
	go func() {
		clock.Sleep(time.Minute)
		close(done)
	}()
	clock.AwaitTimers(1)
	clock.Advance(59 * time.Second)
	select {
	case <-done:
		t.Fatal("Sleep returned before its deadline")
	default:
	}
	clock.Advance(time.Second)
	<-done
}

func TestAwaitTimersReturnsWhenEnoughAreWaiting(t *testing.T) {
	clock := NewClock()
	clock.After(time.Second)
	clock.After(time.Hour)
	clock.AwaitTimers(2)
	if got := clock.PendingTimers(); got != 2 {
		t.Fatalf("PendingTimers() = %d, want 2", got)
	}
}

// assertTimeReceived fails unless a value is ready on ch and equals want.
func assertTimeReceived(t *testing.T, ch <-chan time.Time, want time.Time) {
	t.Helper()
	select {
	case got := <-ch:
		if !got.Equal(want) {
			t.Fatalf("received %v, want %v", got, want)
		}
	default:
		t.Fatalf("nothing received, want %v", want)
	}
}

// assertNothingReceived fails if a value is ready on ch.
func assertNothingReceived(t *testing.T, ch <-chan time.Time) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("received %v, want nothing", got)
	default:
	}
}

func TestJumpFiresOverdueTimersOnceAtWakeWithoutBurst(t *testing.T) {
	clock := NewClock()
	ticks := clock.NewTicker(time.Second)
	defer ticks.Stop()
	later := clock.After(time.Hour)

	clock.Jump(time.Hour)
	assertTimeReceived(t, ticks.C(), epoch.Add(time.Hour))
	assertTimeReceived(t, later, epoch.Add(time.Hour))
	assertNothingReceived(t, ticks.C())

	clock.Advance(time.Second)
	assertTimeReceived(t, ticks.C(), epoch.Add(time.Hour+time.Second))
}
