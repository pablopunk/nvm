package clipboard

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Cases ported from 1.x/electron/clipboard/clipboard-history.test.ts, history-level only. Each test
// names the 1.x case it ports.

// TestRememberPrependsItem ports 'rememberClipboardItem prepends item to history'.
func TestRememberPrependsItem(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:hash123", Kind: KindText, Text: "hello"})
	entries := h.history.Find(Query{})
	if len(entries) != 1 || entries[0].ID != "text:hash123" {
		t.Fatalf("entries = %+v, want one text:hash123", entries)
	}
}

// TestRememberDeduplicatesByID ports 'rememberClipboardItem deduplicates by id'.
func TestRememberDeduplicatesByID(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:hash123", Kind: KindText, Text: "hello"})
	h.mustRemember(Entry{ID: "text:hash123", Kind: KindText, Text: "hello updated"})
	entries := h.history.Find(Query{})
	if len(entries) != 1 || entries[0].Text != "hello updated" {
		t.Fatalf("entries = %+v, want one entry with the updated text", entries)
	}
}

// TestRememberIgnoresInvalidEntries ports 'rememberClipboardItem ignores null/undefined'. The zero entry has
// no ID and no kind, so it is refused and nothing is stored.
func TestRememberIgnoresInvalidEntries(t *testing.T) {
	h := newHarness(t)
	if err := h.history.Remember(Entry{}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("Remember(zero) = %v, want ErrInvalidEntry", err)
	}
	if got := h.history.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0", got)
	}
}

// TestFindReturnsAllEntriesInOrder ports 'clipboardHistorySnapshot returns all entries'. The 1.x test pushed
// entries straight into the array; here the newest entry is remembered last, so the order matches.
func TestFindReturnsAllEntriesInOrder(t *testing.T) {
	h := newHarness(t)
	base := h.clock.Now()
	h.mustRemember(Entry{ID: "image:c", Kind: KindImage, ImageRef: "c.png", CreatedAt: base.Add(-3 * time.Minute)})
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "b", CreatedAt: base.Add(-2 * time.Minute)})
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a", CreatedAt: base.Add(-time.Minute)})
	entries := h.history.Find(Query{})
	if len(entries) != 3 || entries[0].ID != "text:a" || entries[2].ID != "image:c" {
		t.Fatalf("order = %v, want text:a first and image:c last", ids(entries))
	}
}

// TestFindFiltersByKind ports 'clipboardHistorySnapshot filters by type'.
func TestFindFiltersByKind(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	h.mustRemember(Entry{ID: "image:b", Kind: KindImage, ImageRef: "b.png"})
	entries := h.history.Find(Query{Kinds: []Kind{KindImage}})
	if len(entries) != 1 || entries[0].ID != "image:b" {
		t.Fatalf("entries = %v, want image:b only", ids(entries))
	}
}

// TestFindFiltersByQuery ports 'clipboardHistorySnapshot filters by query'.
func TestFindFiltersByQuery(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "goodbye"})
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "hello world"})
	entries := h.history.Search("hello")
	if len(entries) != 1 || entries[0].ID != "text:a" {
		t.Fatalf("entries = %v, want text:a only", ids(entries))
	}
}

// TestFindRespectsLimit ports 'clipboardHistorySnapshot respects limit'.
func TestFindRespectsLimit(t *testing.T) {
	h := newHarness(t)
	for i := range 10 {
		h.mustRemember(Entry{ID: fmt.Sprintf("text:%d", i), Kind: KindText, Text: fmt.Sprintf("item %d", i)})
	}
	if got := len(h.history.Find(Query{Limit: 3})); got != 3 {
		t.Fatalf("Find(limit 3) returned %d entries", got)
	}
}

// TestSearchIsCaseInsensitiveAndTrimmed covers the search rule: the query is trimmed and matched against the
// text, kind and paths, ignoring case. Ported from the 1.x substring rule.
func TestSearchIsCaseInsensitiveAndTrimmed(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "Hello World"})
	h.mustRemember(Entry{ID: "files:b", Kind: KindFiles, Paths: []string{"/home/u/Report.pdf"}, Text: "/home/u/Report.pdf"})
	if got := ids(h.history.Search("  WORLD ")); len(got) != 1 || got[0] != "text:a" {
		t.Fatalf("Search(world) = %v", got)
	}
	if got := ids(h.history.Search("report")); len(got) != 1 || got[0] != "files:b" {
		t.Fatalf("Search(report) = %v", got)
	}
}

// TestRemoveSingleItem ports 'removeClipboardHistoryByAction removes a single item'.
func TestRemoveSingleItem(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "b"})
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	removed, err := h.history.Remove(RangeQuery{Range: RangeItem, ItemID: "text:a"})
	if err != nil || removed != 1 {
		t.Fatalf("Remove = %d, %v; want 1, nil", removed, err)
	}
	if got := ids(h.history.Find(Query{})); len(got) != 1 || got[0] != "text:b" {
		t.Fatalf("remaining = %v", got)
	}
}

// TestRemoveByIDs ports 'removeClipboardHistoryByAction removes by ids'.
func TestRemoveByIDs(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"text:c", "text:b", "text:a"} {
		h.mustRemember(Entry{ID: id, Kind: KindText, Text: id})
	}
	removed, err := h.history.Remove(RangeQuery{Range: RangeIDs, ItemIDs: []string{"text:a", "text:c"}})
	if err != nil || removed != 2 {
		t.Fatalf("Remove = %d, %v; want 2, nil", removed, err)
	}
	if got := ids(h.history.Find(Query{})); len(got) != 1 || got[0] != "text:b" {
		t.Fatalf("remaining = %v", got)
	}
}

// TestRemoveAll ports 'removeClipboardHistoryByAction removes all'.
func TestRemoveAll(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "b"})
	removed, err := h.history.Remove(RangeQuery{Range: RangeAll})
	if err != nil || removed != 2 || h.history.Len() != 0 {
		t.Fatalf("Remove(all) = %d, %v, len %d", removed, err, h.history.Len())
	}
}

// TestRemoveInTimeWindows ports 'removeClipboardHistoryByAction removes entries in the selected time range'.
// Each window removes the entry copied inside it and keeps the one copied before it.
func TestRemoveInTimeWindows(t *testing.T) {
	windows := []struct {
		name     Range
		duration time.Duration
	}{
		{RangeLast5Minutes, 5 * time.Minute},
		{RangeLastHour, time.Hour},
		{RangeLastDay, 24 * time.Hour},
		{RangeLastWeek, 7 * 24 * time.Hour},
	}
	for _, window := range windows {
		t.Run(string(window.name), func(t *testing.T) {
			h := newHarness(t)
			now := h.clock.Now()
			h.mustRemember(Entry{ID: "text:outside", Kind: KindText, Text: "outside", CreatedAt: now.Add(-window.duration * 2)})
			h.mustRemember(Entry{ID: "text:inside", Kind: KindText, Text: "inside", CreatedAt: now.Add(-window.duration / 2)})
			removed, err := h.history.Remove(RangeQuery{Range: window.name})
			if err != nil || removed != 1 {
				t.Fatalf("Remove(%s) = %d, %v; want 1", window.name, removed, err)
			}
			if got := ids(h.history.Find(Query{})); len(got) != 1 || got[0] != "text:outside" {
				t.Fatalf("remaining = %v, want text:outside", got)
			}
		})
	}
}

// TestRemoveOlderThan covers the older-than range: entries copied before now minus the duration go.
func TestRemoveOlderThan(t *testing.T) {
	h := newHarness(t)
	now := h.clock.Now()
	h.mustRemember(Entry{ID: "text:old", Kind: KindText, Text: "old", CreatedAt: now.Add(-3 * time.Hour)})
	h.mustRemember(Entry{ID: "text:new", Kind: KindText, Text: "new", CreatedAt: now.Add(-time.Minute)})
	removed, err := h.history.Remove(RangeQuery{Range: RangeOlderThan, OlderThan: 2 * time.Hour})
	if err != nil || removed != 1 {
		t.Fatalf("Remove(older-than) = %d, %v", removed, err)
	}
	if got := ids(h.history.Find(Query{})); len(got) != 1 || got[0] != "text:new" {
		t.Fatalf("remaining = %v", got)
	}
}

// TestRemoveNoMatchChangesNothing ports 'removeClipboardHistoryByAction no-ops on no match'.
func TestRemoveNoMatchChangesNothing(t *testing.T) {
	h := newHarness(t)
	removed, err := h.history.Remove(RangeQuery{Range: RangeItem, ItemID: "text:nonexistent"})
	if err != nil || removed != 0 || h.history.Len() != 0 {
		t.Fatalf("Remove = %d, %v, len %d", removed, err, h.history.Len())
	}
	if h.persist.Saves() != 0 {
		t.Fatalf("a no-op removal saved %d times", h.persist.Saves())
	}
}

// TestRemovedMessage ports 'clipboardHistoryRemovedMessage singular vs plural'.
func TestRemovedMessage(t *testing.T) {
	if got := RemovedMessage(1); got != "Removed 1 clipboard item" {
		t.Fatalf("RemovedMessage(1) = %q", got)
	}
	if got := RemovedMessage(5); got != "Removed 5 clipboard items" {
		t.Fatalf("RemovedMessage(5) = %q", got)
	}
}

// TestGetRetrievesItemByID ports 'clipboardHistoryGet retrieves item by id'.
func TestGetRetrievesItemByID(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "b"})
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	entry, ok := h.history.Get("text:a")
	if !ok || entry.ID != "text:a" {
		t.Fatalf("Get(text:a) = %+v, %v", entry, ok)
	}
}

// TestGetReturnsNothingForUnknownID ports 'clipboardHistoryGet returns null for unknown id'.
func TestGetReturnsNothingForUnknownID(t *testing.T) {
	h := newHarness(t)
	if _, ok := h.history.Get("text:none"); ok {
		t.Fatal("Get(unknown) found an entry")
	}
}

// TestConcealedEntryIsRefused is the history half of the concealed rule: an entry marked concealed is never
// stored, saved or announced.
func TestConcealedEntryIsRefused(t *testing.T) {
	h := newHarness(t)
	changes, cancel := h.history.Subscribe()
	defer cancel()
	err := h.history.Remember(Entry{ID: "text:x", Kind: KindText, Text: "hunter2", Concealed: true})
	if !errors.Is(err, ErrConcealed) {
		t.Fatalf("Remember(concealed) = %v, want ErrConcealed", err)
	}
	if h.history.Len() != 0 || h.persist.Saves() != 0 {
		t.Fatalf("concealed entry was stored (len %d, saves %d)", h.history.Len(), h.persist.Saves())
	}
	select {
	case <-changes:
		t.Fatal("concealed entry was announced")
	default:
	}
}

func ids(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID)
	}
	return out
}

// fakeClock returns a clock for tests that open a history without the harness.
func fakeClock() platform.Clock {
	return fake.NewClock()
}

// TestClearRemovesEveryEntry covers the clear action: it removes every entry and reports how many went.
func TestClearRemovesEveryEntry(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	h.mustRemember(Entry{ID: "text:b", Kind: KindText, Text: "b"})
	removed, err := h.history.Clear()
	if err != nil || removed != 2 || h.history.Len() != 0 {
		t.Fatalf("Clear = %d, %v; Len %d", removed, err, h.history.Len())
	}
}
