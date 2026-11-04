package child

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// memoryKV is an in-memory memoKV that signals each write, so a test can wait for a background refresh.
type memoryKV struct {
	mu     sync.Mutex
	values map[string]json.RawMessage
	wrote  chan string
}

func newMemoryKV() *memoryKV {
	return &memoryKV{values: map[string]json.RawMessage{}, wrote: make(chan string, 16)}
}

func (k *memoryKV) read(key string) (json.RawMessage, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	value, ok := k.values[key]
	return value, ok, nil
}

func (k *memoryKV) write(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	k.mu.Lock()
	k.values[key] = raw
	k.mu.Unlock()
	k.wrote <- key
	return nil
}

func newTestMemo(clock *fake.Clock) (memoCore, *memoryKV) {
	kv := newMemoryKV()
	return memoCore{kv: kv, clock: clock, flights: newFlightGroup()}, kv
}

func TestMemoReturnsFreshValuesAndReloadsAfterTTL(t *testing.T) {
	clock := fake.NewClock()
	memo, _ := newTestMemo(clock)
	loads := 0
	load := func() (any, error) {
		loads++
		return loads * 10, nil
	}
	var got int
	if err := memo.Memo("weather", time.Minute, &got, load); err != nil || got != 10 {
		t.Fatalf("first memo = %d (%v); want 10", got, err)
	}
	if err := memo.Memo("weather", time.Minute, &got, load); err != nil || got != 10 || loads != 1 {
		t.Fatalf("second memo = %d after %d loads; want the stored value", got, loads)
	}
	clock.Advance(61 * time.Second)
	if err := memo.Memo("weather", time.Minute, &got, load); err != nil || got != 20 || loads != 2 {
		t.Fatalf("memo after ttl = %d after %d loads; want a reload", got, loads)
	}
}

func TestMemoStaleServesStaleAndRefreshesInTheBackground(t *testing.T) {
	clock := fake.NewClock()
	memo, kv := newTestMemo(clock)
	var got string
	if err := memo.MemoStale("feed", time.Minute, time.Minute, &got, func() (any, error) { return "v1", nil }); err != nil {
		t.Fatalf("seed: %v", err)
	}
	<-kv.wrote
	clock.Advance(90 * time.Second)
	refreshed := make(chan struct{})
	if err := memo.MemoStale("feed", time.Minute, time.Minute, &got, func() (any, error) {
		close(refreshed)
		return "v2", nil
	}); err != nil || got != "v1" {
		t.Fatalf("stale read = %q (%v); want the stale value", got, err)
	}
	<-refreshed
	<-kv.wrote
	if err := memo.MemoStale("feed", time.Minute, time.Minute, &got, func() (any, error) { return "never", nil }); err != nil || got != "v2" {
		t.Fatalf("after refresh = %q; want v2", got)
	}
}

func TestMemoLoadErrorIsReturnedAndNothingIsStored(t *testing.T) {
	memo, kv := newTestMemo(fake.NewClock())
	var got string
	err := memo.Memo("broken", time.Minute, &got, func() (any, error) { return nil, errors.New("boom") })
	if err == nil {
		t.Fatal("a failing load returned no error")
	}
	if len(kv.values) != 0 {
		t.Fatalf("a failed load stored %d entries", len(kv.values))
	}
}

func TestProcessCacheHonoursTTLAndStale(t *testing.T) {
	clock := fake.NewClock()
	cache := newProcessCache(clock)
	cache.Set("a", "value", time.Minute)
	if got, ok := cache.Get("a"); !ok || got != "value" {
		t.Fatalf("fresh get = %v, %v", got, ok)
	}
	clock.Advance(2 * time.Minute)
	if _, ok := cache.Get("a"); ok {
		t.Fatal("an expired entry is still fresh")
	}
	if got, ok := cache.Stale("a"); !ok || got != "value" {
		t.Fatalf("stale get = %v, %v; want the expired value", got, ok)
	}
	if cache.Has("a") {
		t.Fatal("Has reported an expired entry")
	}
}

func TestProcessCacheIsBoundedAndInvalidates(t *testing.T) {
	clock := fake.NewClock()
	cache := newProcessCache(clock)
	for i := 0; i <= maxCacheEntries; i++ {
		cache.Set(string(rune('A'+i%26))+time.Duration(i).String(), i, 0)
		clock.Advance(time.Millisecond)
	}
	if len(cache.entries) != maxCacheEntries {
		t.Fatalf("cache holds %d entries; want %d", len(cache.entries), maxCacheEntries)
	}
	cache.Set("keep", 1, 0)
	cache.Invalidate("keep")
	if cache.Has("keep") {
		t.Fatal("invalidated key still present")
	}
	cache.Invalidate("")
	if len(cache.entries) != 0 {
		t.Fatalf("invalidate all left %d entries", len(cache.entries))
	}
}
