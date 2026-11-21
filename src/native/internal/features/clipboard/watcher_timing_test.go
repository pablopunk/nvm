package clipboard

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// Watcher timing: change bursts coalesce into one read, the polling backoff hits exact times, the wait
// resets after a change, and every loop stops on cancellation.

// TestBurstOfCopiesIsReadOnce: a burst of change notifications coalesces into one read after the debounce, and
// the read sees the last copy.
func TestBurstOfCopiesIsReadOnce(t *testing.T) {
	h := newHarness(t)
	h.watcher.prime()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.watcher.runOnChange(ctx)
		close(done)
	}()
	changes, stop := h.history.Subscribe()
	defer stop()
	for i := range 10 {
		mustWrite(t, h, fmt.Sprintf("burst %d", i))
	}
	h.clock.AwaitTimers(1)
	h.clock.Advance(changeDebounce)
	<-changes
	cancel()
	<-done
	if got := h.texts(); len(got) != 1 || got[0] != "burst 9" {
		t.Fatalf("texts = %v, want only the last copy of the burst", got)
	}
}

// TestPollingBackoffSchedule proves the exact poll times: the first poll 150 ms after the baseline, then the wait
// doubles on each idle poll up to one second.
func TestPollingBackoffSchedule(t *testing.T) {
	h := newHarness(t)
	rec := &formatRecorder{Clipboard: h.clip, clock: h.clock}
	w := NewWatcher(WatcherDeps{
		Clipboard: rec, Files: h.files, Capabilities: h.capsPoll, Clock: h.clock,
		History: h.history, Images: h.images, Log: h.log,
	})
	w.prime()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.runPolling(ctx)
		close(done)
	}()
	h.clock.AwaitTimers(1)
	for _, wait := range []time.Duration{150, 300, 600, 1000, 1000} {
		h.clock.Advance(wait * time.Millisecond)
		h.clock.AwaitTimers(1)
	}
	cancel()
	<-done
	want := []time.Duration{0, 150, 450, 1050, 2050, 3050}
	if len(rec.calls) != len(want) {
		t.Fatalf("polls = %d, want %d", len(rec.calls), len(want))
	}
	start := rec.calls[0]
	for i, at := range rec.calls {
		if got := at.Sub(start); got != want[i]*time.Millisecond {
			t.Fatalf("poll %d at %v, want %v", i, got, want[i]*time.Millisecond)
		}
	}
}

// TestPollingResetsAfterChange: a change returns the wait to the fast cadence.
func TestPollingResetsAfterChange(t *testing.T) {
	if got := nextPollInterval(1000*time.Millisecond, true); got != pollMin {
		t.Fatalf("after a change the wait is %v, want %v", got, pollMin)
	}
	if got := nextPollInterval(pollMin, false); got != 2*pollMin {
		t.Fatalf("idle wait = %v, want %v", got, 2*pollMin)
	}
	if got := nextPollInterval(800*time.Millisecond, false); got != pollMax {
		t.Fatalf("idle wait capped at %v, want %v", got, pollMax)
	}
}

// TestShutdownStopsTheLoop: cancelling the context ends the loop, whether it waits for a notification, a poll
// timer, or runs under Start, and nothing keeps running.
func TestShutdownStopsTheLoop(t *testing.T) {
	t.Run("change notification loop", func(t *testing.T) {
		h := newHarness(t)
		h.watcher.prime()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			h.watcher.runOnChange(ctx)
			close(done)
		}()
		cancel()
		<-done
	})
	t.Run("polling loop", func(t *testing.T) {
		h := newHarness(t)
		w := h.pollingWatcher()
		w.prime()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.runPolling(ctx)
			close(done)
		}()
		h.clock.AwaitTimers(1)
		cancel()
		<-done
	})
	t.Run("started watcher", func(t *testing.T) {
		h := newHarness(t)
		baseline := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		done := h.watcher.Start(ctx)
		cancel()
		<-done
		waitForGoroutines(t, baseline)
	})
}

// waitForGoroutines yields until the goroutine count is back to baseline or below. It fails after a bounded
// number of yields, so a goroutine that outlives shutdown fails the test instead of hanging it.
func waitForGoroutines(t *testing.T, baseline int) {
	t.Helper()
	for range 100000 {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("goroutines = %d after shutdown, baseline %d", runtime.NumGoroutine(), baseline)
}
