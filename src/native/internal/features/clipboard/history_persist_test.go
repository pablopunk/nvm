package clipboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// History persistence cases: round trips, read-only refusal of newer snapshots, load-time filtering, save
// failures, and change subscriptions. Ported behavior is noted per test where the 1.x suite has a counterpart.

// TestPersistenceRoundTrip stores a history and loads it back in the same order with the same fields.
func TestPersistenceRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a", SourceApp: "Notes"})
	h.mustRemember(Entry{ID: "image:b", Kind: KindImage, ImageRef: strings.Repeat("b", 40) + ".png", Width: 3, Height: 2, Bytes: 9})
	reopened, err := Open(h.persist, h.clock, Retention{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := reopened.Find(Query{})
	if len(got) != 2 || got[0].ID != "image:b" || got[1].SourceApp != "Notes" || got[0].Width != 3 {
		t.Fatalf("reloaded = %+v", got)
	}
}

// TestNewerSnapshotIsReadOnly: a snapshot from a newer build is never overwritten.
func TestNewerSnapshotIsReadOnly(t *testing.T) {
	persist := NewMemoryPersistence()
	persist.SetStored(Snapshot{Version: SnapshotVersion + 1, Entries: []Entry{{ID: "text:x", Kind: KindText, Text: "x"}}})
	h, err := Open(persist, fakeClock(), Retention{})
	if !errors.Is(err, ErrNewerSnapshot) || h == nil {
		t.Fatalf("Open = %v, %v; want the read-only history and ErrNewerSnapshot", h, err)
	}
	if err := h.Remember(Entry{ID: "text:y", Kind: KindText, Text: "y"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Remember on read-only = %v", err)
	}
	if h.Trusted() {
		t.Fatal("read-only history reports itself trusted for garbage collection")
	}
	if persist.Saves() != 0 || persist.Stored().Version != SnapshotVersion+1 {
		t.Fatal("the newer snapshot was overwritten")
	}
}

// TestLoadDropsConcealedAndInvalidEntries: a snapshot written by an older build with bad entries loads
// only the valid ones.
func TestLoadDropsConcealedAndInvalidEntries(t *testing.T) {
	clock := fakeClock()
	now := clock.Now()
	persist := NewMemoryPersistence()
	persist.SetStored(Snapshot{Version: SnapshotVersion, Entries: []Entry{
		{ID: "text:ok", Kind: KindText, Text: "ok", CreatedAt: now},
		{ID: "text:secret", Kind: KindText, Text: "secret", Concealed: true, CreatedAt: now},
		{ID: "", Kind: KindText, Text: "no id", CreatedAt: now},
		{ID: "text:bad", Kind: Kind("video"), Text: "bad kind", CreatedAt: now},
		{ID: "text:ok", Kind: KindText, Text: "duplicate", CreatedAt: now},
	}})
	h, err := Open(persist, clock, Retention{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := ids(h.Find(Query{})); len(got) != 1 || got[0] != "text:ok" {
		t.Fatalf("loaded = %v, want only text:ok", got)
	}
}

// TestConcealedFieldIsNeverSerialized: the JSON of a saved snapshot has no concealed flag.
func TestConcealedFieldIsNeverSerialized(t *testing.T) {
	data, err := json.Marshal(Snapshot{Version: SnapshotVersion, Entries: []Entry{{ID: "text:a", Kind: KindText, Text: "a", Concealed: true}}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "oncealed") {
		t.Fatalf("snapshot JSON mentions the concealed flag: %s", data)
	}
}

// TestSaveFailureKeepsMemoryAndReturnsError: a failed save leaves the change in memory and tells the caller.
func TestSaveFailureKeepsMemoryAndReturnsError(t *testing.T) {
	h := newHarness(t)
	h.persist.FailNextSave(errors.New("disk full"))
	err := h.history.Remember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	if err == nil {
		t.Fatal("Remember returned nil after a failed save")
	}
	if h.history.Len() != 1 {
		t.Fatalf("Len = %d; the entry should stay in memory", h.history.Len())
	}
	if err := h.history.Remember(Entry{ID: "text:b", Kind: KindText, Text: "b"}); err != nil {
		t.Fatalf("next save: %v", err)
	}
	if got := len(h.persist.Stored().Entries); got != 2 {
		t.Fatalf("stored %d entries after the retry, want 2", got)
	}
}

// TestSubscribeCoalescesNotifications: a burst of changes produces one pending notification.
func TestSubscribeCoalescesNotifications(t *testing.T) {
	h := newHarness(t)
	changes, cancel := h.history.Subscribe()
	defer cancel()
	for i := range 5 {
		h.mustRemember(Entry{ID: fmt.Sprintf("text:%d", i), Kind: KindText, Text: fmt.Sprintf("%d", i)})
	}
	<-changes
	select {
	case <-changes:
		t.Fatal("a second notification was queued for a coalesced burst")
	default:
	}
}

// TestSubscriptionStopsAfterCancel: a cancelled subscriber gets nothing more.
func TestSubscriptionStopsAfterCancel(t *testing.T) {
	h := newHarness(t)
	changes, cancel := h.history.Subscribe()
	cancel()
	h.mustRemember(Entry{ID: "text:a", Kind: KindText, Text: "a"})
	select {
	case <-changes:
		t.Fatal("notification after cancel")
	default:
	}
}
