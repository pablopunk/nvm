package clipboard

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"
)

// Retention cases and the randomized and concurrent checks of History: the entry cap, the byte caps and
// expiry, the invariants held under a seeded run of operations, and race-freedom under concurrent use.

// TestRetentionEntryCapDropsOldestUnpinned: the cap keeps the newest unpinned entries.
func TestRetentionEntryCapDropsOldestUnpinned(t *testing.T) {
	h := newHarness(t)
	h.history.policy = Retention{MaxEntries: 3, MaxAge: 24 * time.Hour, MaxImageBytes: DefaultMaxImageBytes, MaxMemoryBytes: DefaultMaxMemoryBytes}
	for i := range 6 {
		h.mustRemember(Entry{ID: fmt.Sprintf("text:%d", i), Kind: KindText, Text: fmt.Sprintf("%d", i)})
	}
	if got := ids(h.history.Find(Query{})); strings.Join(got, ",") != "text:5,text:4,text:3" {
		t.Fatalf("kept %v, want the three newest", got)
	}
}

// TestRetentionByteCapsEvictImagesAndText: the image byte cap evicts images, the memory cap evicts text.
func TestRetentionByteCapsEvictImagesAndText(t *testing.T) {
	h := newHarness(t)
	h.history.policy = Retention{MaxEntries: 100, MaxAge: 24 * time.Hour, MaxImageBytes: 250, MaxMemoryBytes: 10}
	for i := range 3 {
		h.mustRemember(Entry{ID: fmt.Sprintf("image:%d", i), Kind: KindImage, ImageRef: fmt.Sprintf("%040d.png", i), Bytes: 100})
	}
	if got := h.history.Stats().ImageBytes; got > 250 {
		t.Fatalf("image bytes = %d, over the cap", got)
	}
	h.mustRemember(Entry{ID: "text:long", Kind: KindText, Text: strings.Repeat("x", 9)})
	h.mustRemember(Entry{ID: "text:longer", Kind: KindText, Text: strings.Repeat("y", 9)})
	if got := h.history.Stats().MemoryBytes; got > 10 {
		t.Fatalf("memory bytes = %d, over the cap", got)
	}
}

// TestRetentionExpiresByAge: unpinned entries older than MaxAge go on enforcement.
func TestRetentionExpiresByAge(t *testing.T) {
	h := newHarness(t)
	h.history.policy = Retention{MaxEntries: 100, MaxAge: time.Hour, MaxImageBytes: DefaultMaxImageBytes, MaxMemoryBytes: DefaultMaxMemoryBytes}
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	h.clock.Advance(2 * time.Hour)
	removed, err := h.history.Enforce()
	if err != nil || removed != 1 {
		t.Fatalf("Enforce = %d, %v", removed, err)
	}
}

// TestHistoryPropertiesHoldUnderRandomOperations runs a seeded sequence of operations and checks the
// invariants after each step: the unpinned cap holds, pinned entries survive, totals match the entries,
// IDs are unique, and the saved snapshot equals the live list.
func TestHistoryPropertiesHoldUnderRandomOperations(t *testing.T) {
	h := newHarness(t)
	policy := Retention{MaxEntries: 12, MaxAge: 3 * time.Hour, MaxImageBytes: 5000, MaxMemoryBytes: 4000}
	h.history.policy = policy
	rng := rand.New(rand.NewPCG(1, 2))
	for step := range 1500 {
		switch rng.IntN(5) {
		case 0, 1:
			h.rememberRandomEntry(rng, step)
		case 2:
			h.clock.Advance(time.Duration(rng.IntN(40)) * time.Minute)
			if _, err := h.history.Enforce(); err != nil {
				t.Fatalf("step %d: Enforce: %v", step, err)
			}
		case 3:
			h.clock.Advance(time.Duration(rng.IntN(5)) * time.Second)
		case 4:
			id := h.randomID(rng)
			if id != "" {
				if _, err := h.history.Delete(id); err != nil {
					t.Fatalf("step %d: Delete: %v", step, err)
				}
			}
		}
		h.checkInvariants(t, step, policy)
	}
}

// rememberRandomEntry stores a text or image entry of random size.
func (h *harness) rememberRandomEntry(rng *rand.Rand, step int) {
	h.t.Helper()
	if rng.IntN(3) == 0 {
		name := fmt.Sprintf("%040x.png", step)
		if err := h.history.Remember(Entry{ID: ImageID(name), Kind: KindImage, ImageRef: name, Bytes: int64(100 + rng.IntN(900))}); err != nil {
			h.t.Fatalf("step %d: Remember image: %v", step, err)
		}
		return
	}
	text := strings.Repeat("t", 1+rng.IntN(400))
	if err := h.history.Remember(Entry{ID: fmt.Sprintf("text:%d", step), Kind: KindText, Text: text}); err != nil {
		h.t.Fatalf("step %d: Remember text: %v", step, err)
	}
}

// randomID returns the ID of a random live entry, or "" when the history is empty.
func (h *harness) randomID(rng *rand.Rand) string {
	entries := h.history.Find(Query{Limit: 1 << 20})
	if len(entries) == 0 {
		return ""
	}
	return entries[rng.IntN(len(entries))].ID
}

// liveOrder returns the entries in the history's internal order, which is copy order regardless of pins.
func (h *harness) liveOrder() []Entry {
	h.history.mu.Lock()
	defer h.history.mu.Unlock()
	out := make([]Entry, 0, len(h.history.order))
	for _, r := range h.history.order {
		out = append(out, r.entry)
	}
	return out
}

// checkInvariants asserts the history properties after one step.
func (h *harness) checkInvariants(t *testing.T, step int, policy Retention) {
	t.Helper()
	entries := h.liveOrder()
	seen := map[string]bool{}
	var memory, imageBytes int64
	for _, e := range entries {
		if seen[e.ID] {
			t.Fatalf("step %d: duplicate id %s", step, e.ID)
		}
		seen[e.ID] = true
		memory += e.memoryBytes()
		imageBytes += e.imageBytes()
	}
	if len(entries) > policy.MaxEntries {
		t.Fatalf("step %d: %d entries, cap %d", step, len(entries), policy.MaxEntries)
	}
	stats := h.history.Stats()
	if stats.Entries != len(entries) || stats.MemoryBytes != memory || stats.ImageBytes != imageBytes {
		t.Fatalf("step %d: totals %+v do not match entries (%d, %d, %d)", step, stats, len(entries), memory, imageBytes)
	}
	stored := h.persist.Stored().Entries
	if len(stored) != len(entries) {
		t.Fatalf("step %d: saved %d entries, live %d", step, len(stored), len(entries))
	}
	for i := range stored {
		if stored[i].ID != entries[i].ID {
			t.Fatalf("step %d: saved order differs from the live order at %d", step, i)
		}
	}
}

// TestHistoryConcurrentUseIsRaceFree runs readers, writers, pins, enforcement and subscribers together. Run
// under -race, it proves the locks cover every path.
func TestHistoryConcurrentUseIsRaceFree(t *testing.T) {
	h := newHarness(t)
	h.history.policy = Retention{MaxEntries: 20, MaxAge: time.Hour, MaxImageBytes: DefaultMaxImageBytes, MaxMemoryBytes: DefaultMaxMemoryBytes}
	changes, cancel := h.history.Subscribe()
	defer cancel()
	var wg sync.WaitGroup
	for worker := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(worker), 7))
			for step := range 200 {
				switch rng.IntN(5) {
				case 0:
					_ = h.history.Remember(Entry{ID: fmt.Sprintf("text:%d:%d", worker, step), Kind: KindText, Text: "concurrent"})
				case 1:
					_ = h.history.Search("conc")
				case 2:
					_, _ = h.history.Clear()
				case 3:
					_, _ = h.history.Enforce()
				case 4:
					_ = h.history.HasImage("none.png")
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			select {
			case <-changes:
			default:
			}
		}
	}()
	wg.Wait()
	entries := h.history.Find(Query{Limit: 1 << 20})
	if got := h.history.Stats().Entries; got != len(entries) {
		t.Fatalf("Stats().Entries = %d, Find returned %d", got, len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.ID] {
			t.Fatalf("duplicate id %s after concurrent use", e.ID)
		}
		seen[e.ID] = true
	}
}

// ids returns the IDs of entries in order.
