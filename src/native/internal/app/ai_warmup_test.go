package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type warmupHarness struct {
	clock    *fake.Clock
	signedIn bool
	warmups  int
	err      error
	queued   []func(ctx context.Context)
	mu       sync.Mutex
	warmup   *aiWarmup
}

func newWarmupHarness() *warmupHarness {
	h := &warmupHarness{clock: fake.NewClock(), signedIn: true}
	h.warmup = &aiWarmup{
		clock:    h.clock,
		signedIn: func() bool { return h.signedIn },
		warm:     func(context.Context) error { h.warmups++; return h.err },
		start:    func(fn func(ctx context.Context)) { h.queued = append(h.queued, fn) },
	}
	return h
}

// drain runs the queued warm-ups, as the background goroutine would.
func (h *warmupHarness) drain() {
	queued := h.queued
	h.queued = nil
	for _, fn := range queued {
		fn(context.Background())
	}
}

func TestWarmupStartsOffTheCallerAndOnlyWhenSignedIn(t *testing.T) {
	h := newWarmupHarness()
	h.signedIn = false
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 0 {
		t.Fatal("warmed while signed out")
	}
	h.signedIn = true
	h.warmup.onPaletteShown()
	if h.warmups != 0 {
		t.Fatal("the warm-up ran on the caller's goroutine, before the palette could take input")
	}
	h.drain()
	if h.warmups != 1 {
		t.Fatalf("warmups = %d, want 1", h.warmups)
	}
}

func TestWarmupIsThrottledToTheInterval(t *testing.T) {
	h := newWarmupHarness()
	h.warmup.onPaletteShown()
	h.drain()
	h.clock.Advance(aiWarmInterval - time.Second)
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 1 {
		t.Fatalf("warmups = %d inside the interval, want 1", h.warmups)
	}
	h.clock.Advance(time.Second)
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 2 {
		t.Fatalf("warmups = %d after the interval, want 2", h.warmups)
	}
}

func TestWarmupDoesNotOverlapARunningWarmup(t *testing.T) {
	h := newWarmupHarness()
	h.warmup.onPaletteShown()
	h.clock.Advance(time.Hour)
	h.warmup.onPaletteShown()
	if len(h.queued) != 1 {
		t.Fatalf("queued %d warm-ups while one was running, want 1", len(h.queued))
	}
}

func TestFailedWarmupIsThrottledAndDoesNotBlockLaterOnes(t *testing.T) {
	h := newWarmupHarness()
	h.err = errors.New("route lookup failed")
	h.warmup.onPaletteShown()
	h.drain()
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 1 {
		t.Fatalf("warmups = %d, a failed warm-up must still wait out the interval", h.warmups)
	}
	h.err = nil
	h.clock.Advance(aiWarmInterval)
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 2 {
		t.Fatalf("warmups = %d, want a retry after the interval", h.warmups)
	}
}

func TestSignedOutOpenDoesNotUseUpTheInterval(t *testing.T) {
	h := newWarmupHarness()
	h.signedIn = false
	h.warmup.onPaletteShown()
	h.signedIn = true
	h.warmup.onPaletteShown()
	h.drain()
	if h.warmups != 1 {
		t.Fatalf("warmups = %d, want 1 right after signing in", h.warmups)
	}
}
