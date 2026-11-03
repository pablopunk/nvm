package clipboard

import (
	"fmt"
	"slices"
	"time"
)

// Change tracking for History: every mutation runs under the lock, produces one snapshot, notifies the
// subscribers without blocking, and saves outside the lock in order.

// Subscribe returns a channel that receives after each change and a function that stops the subscription.
// Notifications coalesce: a receive means the history changed at least once since the previous receive, and
// a receiver reads the state it needs with Find or Get.
func (h *History) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	id := h.nextSub
	h.nextSub++
	h.subs[id] = ch
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs, id)
	}
}

// mutate runs change under the lock, then saves the snapshot the change produced. change reports whether
// anything changed; an unchanged history is neither saved nor announced. A read-only history refuses.
func (h *History) mutate(change func(now time.Time) bool) error {
	h.mu.Lock()
	if h.readOnly {
		h.mu.Unlock()
		return ErrReadOnly
	}
	if !change(h.clock.Now()) {
		h.mu.Unlock()
		return nil
	}
	h.seq++
	seq := h.seq
	snapshot := h.snapshotLocked()
	h.notifyLocked()
	h.mu.Unlock()
	return h.save(seq, snapshot)
}

// save stores snapshot unless a later one already went to disk.
func (h *History) save(seq uint64, snapshot Snapshot) error {
	h.saveMu.Lock()
	defer h.saveMu.Unlock()
	if seq <= h.saveSeq {
		return nil
	}
	if err := h.persist.Save(snapshot); err != nil {
		return fmt.Errorf("clipboard: save history: %w", err)
	}
	h.saveSeq = seq
	return nil
}

// snapshotLocked copies the entries into a persistable snapshot. The caller holds h.mu.
func (h *History) snapshotLocked() Snapshot {
	entries := make([]Entry, 0, len(h.order))
	for _, r := range h.order {
		entries = append(entries, r.entry)
	}
	return Snapshot{Version: SnapshotVersion, Entries: entries}
}

// restoreLocked loads saved entries in their saved order, skipping concealed, invalid and duplicate ones.
// The caller holds h.mu and runs on an empty history.
func (h *History) restoreLocked(entries []Entry) {
	for _, e := range entries {
		if e.Concealed || e.ID == "" || !e.Kind.Valid() {
			continue
		}
		if _, dup := h.byID[e.ID]; dup {
			continue
		}
		h.appendLocked(newRecord(e))
	}
}

// appendLocked adds r at the end of the order and updates the indexes and totals. The caller holds h.mu.
func (h *History) appendLocked(r *record) {
	h.order = append(h.order, r)
	h.indexLocked(r, +1)
}

// prependLocked adds e as the newest entry. The caller holds h.mu.
func (h *History) prependLocked(e Entry) {
	r := newRecord(e)
	h.order = slices.Insert(h.order, 0, r)
	h.indexLocked(r, +1)
}

// indexLocked adds (sign +1) or removes (sign -1) r from the ID index, the image reference counts and the
// totals. The caller holds h.mu.
func (h *History) indexLocked(r *record, sign int64) {
	if sign > 0 {
		h.byID[r.entry.ID] = r
	} else {
		delete(h.byID, r.entry.ID)
	}
	if r.entry.Kind == KindImage && r.entry.ImageRef != "" {
		h.imageRefs[r.entry.ImageRef] += int(sign)
		if h.imageRefs[r.entry.ImageRef] <= 0 {
			delete(h.imageRefs, r.entry.ImageRef)
		}
	}
	h.memory += sign * r.memory
	h.imageBytes += sign * r.image
}

// notifyLocked tells each subscriber about a change without blocking. The caller holds h.mu.
func (h *History) notifyLocked() {
	for _, ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
