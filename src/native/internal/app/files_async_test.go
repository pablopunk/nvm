package app

import (
	"context"
	"testing"
	"time"
)

// awaitValue waits for a notification, failing the test if none comes: a guard against a hang, not a sleep.
func awaitValue[T any](t *testing.T, ch <-chan T, label string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not arrive", label)
		var zero T
		return zero
	}
}

// TestAsyncCacheMissReturnsAtOnceAndNotifiesWhenLoaded checks the contract the UI relies on: a miss returns without
// waiting for the load, the load runs on a worker, and its value is stored and announced.
func TestAsyncCacheMissReturnsAtOnceAndNotifiesWhenLoaded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notified := make(chan struct{}, 4)
	c := newAsyncCache[int](ctx, 1, 8, 16, func() { notified <- struct{}{} })
	release := make(chan struct{})

	if _, ok := c.Get("k", func() int { <-release; return 7 }); ok {
		t.Fatal("a miss must report not ready")
	}
	if _, ok := c.Get("k", func() int { return 9 }); ok {
		t.Fatal("a load still running must keep reporting not ready")
	}
	close(release)
	awaitValue(t, notified, "the notification")
	if v, ok := c.Get("k", func() int { t.Fatal("a stored key must not load again"); return 0 }); !ok || v != 7 {
		t.Fatalf("after the load, Get = %d, %v; want 7, true", v, ok)
	}
}

// TestAsyncCacheIsBounded checks that the oldest values are forgotten past the entry bound.
func TestAsyncCacheIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newAsyncCache[int](ctx, 1, 8, 3, nil)
	for i := range 5 {
		c.Put(string(rune('a'+i)), i)
	}
	if got := c.Len(); got != 3 {
		t.Fatalf("entries = %d, want the bound of 3", got)
	}
	if _, ok := c.Get("a", func() int { return 0 }); ok {
		t.Fatal("the oldest entry should have been forgotten")
	}
}

// TestAsyncCacheSurvivesAPanickingLoad checks that a load that panics is not stored and does not stop the pool.
func TestAsyncCacheSurvivesAPanickingLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notified := make(chan struct{}, 4)
	c := newAsyncCache[int](ctx, 1, 8, 16, func() { notified <- struct{}{} })
	c.Get("boom", func() int { panic("bad image") })
	c.Get("ok", func() int { return 3 })
	awaitValue(t, notified, "the notification of the healthy load")
	if v, ok := c.Get("ok", func() int { return 0 }); !ok || v != 3 {
		t.Fatalf("healthy load after a panic = %d, %v", v, ok)
	}
	if _, ok := c.Get("boom", func() int { return 1 }); ok {
		t.Fatal("a panicked load must not be stored")
	}
}
