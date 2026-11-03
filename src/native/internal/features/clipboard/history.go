package clipboard

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Errors returned by History.
var (
	// ErrConcealed refuses an entry copied from a concealed source. Nothing is stored, logged or announced.
	ErrConcealed = errors.New("clipboard: concealed content is never stored")
	// ErrInvalidEntry refuses an entry without an ID or with an unknown kind.
	ErrInvalidEntry = errors.New("clipboard: entry needs an id and a known kind")
	// ErrReadOnly refuses a change to a history loaded read-only.
	ErrReadOnly = errors.New("clipboard: history is read-only")
	// ErrNewerSnapshot is returned by Open when the saved history was written by a newer build.
	ErrNewerSnapshot = errors.New("clipboard: saved history was written by a newer build; it is kept read-only")
)

// record is one entry with the values History derives from it once: its lowercase search key and its
// memory and image sizes.
type record struct {
	entry    Entry
	haystack string
	memory   int64
	image    int64
}

func newRecord(e Entry) *record {
	return &record{
		entry:    e,
		haystack: strings.ToLower(e.Text + " " + string(e.Kind) + " " + strings.Join(e.Paths, " ")),
		memory:   e.memoryBytes(),
		image:    e.imageBytes(),
	}
}

// History holds clipboard entries newest first and persists every change. It is safe for concurrent use.
// Changes are saved outside the lock, in the order they were made; a save that finds a newer one already
// stored is skipped, so the file always ends with the latest state.
type History struct {
	clock   platform.Clock
	persist Persistence
	policy  Retention

	mu         sync.Mutex
	order      []*record
	byID       map[string]*record
	imageRefs  map[string]int
	memory     int64
	imageBytes int64
	seq        uint64
	subs       map[uint64]chan struct{}
	nextSub    uint64
	readOnly   bool

	saveMu  sync.Mutex
	saveSeq uint64
}

// Open loads the saved history and applies retention. With ErrNewerSnapshot the returned History holds
// nothing and refuses every change, so data written by a newer build is never overwritten. Any other load
// error returns no History.
func Open(persist Persistence, clock platform.Clock, policy Retention) (*History, error) {
	h := &History{
		clock:     clock,
		persist:   persist,
		policy:    policy.withDefaults(),
		byID:      map[string]*record{},
		imageRefs: map[string]int{},
		subs:      map[uint64]chan struct{}{},
	}
	snapshot, err := persist.Load()
	if err != nil {
		return nil, fmt.Errorf("clipboard: load history: %w", err)
	}
	if snapshot.Version > SnapshotVersion {
		h.readOnly = true
		return h, ErrNewerSnapshot
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.restoreLocked(snapshot.Entries)
	h.enforceLocked(clock.Now())
	return h, nil
}

// Remember stores e as the newest entry. An entry with the same ID moves to the top, keeping its pinned
// state. The change is saved before Remember returns; a failed save leaves the entry in memory and returns
// the error.
func (h *History) Remember(e Entry) error {
	if e.Concealed {
		return ErrConcealed
	}
	if e.ID == "" || !e.Kind.Valid() {
		return ErrInvalidEntry
	}
	return h.mutate(func(now time.Time) bool {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = now
		}
		if old, ok := h.byID[e.ID]; ok {
			e.Pinned = e.Pinned || old.entry.Pinned
			h.dropLocked(map[*record]bool{old: true})
		}
		h.prependLocked(e)
		h.enforceLocked(now)
		return true
	})
}

// SetPinned pins or unpins the entry with id. Unpinning applies retention again, since the entry is no longer
// protected. It reports whether the entry exists.
func (h *History) SetPinned(id string, pinned bool) (bool, error) {
	found := false
	err := h.mutate(func(now time.Time) bool {
		r, ok := h.byID[id]
		if !ok {
			return false
		}
		found = true
		if r.entry.Pinned == pinned {
			return false
		}
		r.entry.Pinned = pinned
		if !pinned {
			h.enforceLocked(now)
		}
		return true
	})
	return found, err
}

// Delete removes the entry with id. It reports whether the entry existed.
func (h *History) Delete(id string) (bool, error) {
	removed := false
	err := h.mutate(func(time.Time) bool {
		r, ok := h.byID[id]
		if !ok {
			return false
		}
		removed = true
		h.dropLocked(map[*record]bool{r: true})
		return true
	})
	return removed, err
}

// ClearUnpinned removes every unpinned entry and returns how many went.
func (h *History) ClearUnpinned() (int, error) {
	removed := 0
	err := h.mutate(func(time.Time) bool {
		drop := map[*record]bool{}
		for _, r := range h.order {
			if !r.entry.Pinned {
				drop[r] = true
			}
		}
		removed = h.dropLocked(drop)
		return removed > 0
	})
	return removed, err
}

// Remove deletes the entries a range selects and returns how many went. Explicit removal applies to pinned
// entries too: the user asked for it.
func (h *History) Remove(q RangeQuery) (int, error) {
	removed := 0
	err := h.mutate(func(now time.Time) bool {
		drop := map[*record]bool{}
		for _, r := range h.matchLocked(q, now) {
			drop[r] = true
		}
		removed = h.dropLocked(drop)
		return removed > 0
	})
	return removed, err
}

// MarkUsed records that the entry was copied or pasted from history now. It reports whether it exists.
func (h *History) MarkUsed(id string) (bool, error) {
	found := false
	err := h.mutate(func(now time.Time) bool {
		r, ok := h.byID[id]
		if !ok {
			return false
		}
		found = true
		r.entry.UsedAt = now
		return true
	})
	return found, err
}

// Enforce applies retention now, for example after the clock moved, and returns how many entries went.
func (h *History) Enforce() (int, error) {
	removed := 0
	err := h.mutate(func(now time.Time) bool {
		removed = h.enforceLocked(now)
		return removed > 0
	})
	return removed, err
}
