package clipboard

import (
	"fmt"
	"strings"
	"time"
)

// Query selects entries for display. Text is matched case-insensitively as a substring of the entry's
// text, kind and paths, after trimming; Kinds, when set, keeps only those kinds; Limit caps the rows
// (DefaultLimit when zero). Pinned entries come first, then the rest, each in copy order.
type Query struct {
	Text  string
	Kinds []Kind
	Limit int
}

// Stats reports what the history holds, for budgets and diagnostics.
type Stats struct {
	Entries     int
	MemoryBytes int64
	ImageBytes  int64
}

// Find returns the entries the query selects.
func (h *History) Find(q Query) []Entry {
	needle := strings.ToLower(strings.TrimSpace(q.Text))
	kinds := kindSet(q.Kinds)
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Entry, 0, min(limit, len(h.order)))
	for _, pinnedPass := range []bool{true, false} {
		for _, r := range h.order {
			if r.entry.Pinned != pinnedPass {
				continue
			}
			if len(kinds) > 0 && !kinds[r.entry.Kind] {
				continue
			}
			if needle != "" && !strings.Contains(r.haystack, needle) {
				continue
			}
			out = append(out, r.entry)
			if len(out) == limit {
				return out
			}
		}
	}
	return out
}

// Search returns up to DefaultLimit entries whose text, kind or paths contain text, case-insensitively.
func (h *History) Search(text string) []Entry {
	return h.Find(Query{Text: text})
}

// Get returns the entry with id.
func (h *History) Get(id string) (Entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.byID[id]
	if !ok {
		return Entry{}, false
	}
	return r.entry, true
}

// Matching returns the entries a range selects, without removing them.
func (h *History) Matching(q RangeQuery) []Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	records := h.matchLocked(q, h.clock.Now())
	out := make([]Entry, 0, len(records))
	for _, r := range records {
		out = append(out, r.entry)
	}
	return out
}

// Len returns the number of entries.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.order)
}

// Stats returns the entry count and byte totals.
func (h *History) Stats() Stats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Stats{Entries: len(h.order), MemoryBytes: h.memory, ImageBytes: h.imageBytes}
}

// Trusted reports whether the live entries are the whole history. It is false when the saved history was
// written by a newer build, and garbage collection then deletes nothing.
func (h *History) Trusted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.readOnly
}

// HasImage reports whether a live entry references the image file name now. The garbage collector calls it
// again right before each delete, so an image copied during a sweep is kept.
func (h *History) HasImage(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.imageRefs[name] > 0
}

// ImageNames returns every image file name referenced by a live entry or by the saved snapshot, so an image
// that only the saved copy references is kept too. It fails when the saved snapshot cannot be read, and the
// caller must then delete nothing.
func (h *History) ImageNames() (map[string]bool, error) {
	h.mu.Lock()
	names := make(map[string]bool, len(h.imageRefs))
	for name := range h.imageRefs {
		names[name] = true
	}
	h.mu.Unlock()
	snapshot, err := h.persist.Load()
	if err != nil {
		return nil, fmt.Errorf("clipboard: read saved history: %w", err)
	}
	for _, e := range snapshot.Entries {
		for _, name := range []string{imageRef(e), linkRef(e)} {
			if name != "" {
				names[name] = true
			}
		}
	}
	return names, nil
}

// matchLocked returns the records a range selects, in copy order. The caller holds h.mu.
func (h *History) matchLocked(q RangeQuery, now time.Time) []*record {
	kinds := kindSet(q.Kinds)
	ids := map[string]bool{}
	switch q.Range {
	case RangeItem:
		ids[q.ItemID] = true
	case RangeIDs:
		for _, id := range q.ItemIDs {
			ids[id] = true
		}
	}
	var out []*record
	for _, r := range h.order {
		if len(kinds) > 0 && !kinds[r.entry.Kind] {
			continue
		}
		if rangeMatches(q, r.entry, ids, now) {
			out = append(out, r)
		}
	}
	return out
}

// rangeMatches reports whether one entry falls in the range. Time windows include entries copied exactly
// at their start, as the 1.x code did.
func rangeMatches(q RangeQuery, e Entry, ids map[string]bool, now time.Time) bool {
	if window, ok := rangeWindows[q.Range]; ok {
		return !e.CreatedAt.Before(now.Add(-window))
	}
	switch q.Range {
	case RangeItem, RangeIDs:
		return ids[e.ID]
	case RangeOlderThan:
		return e.CreatedAt.Before(now.Add(-max(0, q.OlderThan)))
	case RangeAll:
		return true
	}
	return false
}

// kindSet turns a kind list into a lookup set; nil when the list is empty, meaning no kind filter.
func kindSet(kinds []Kind) map[Kind]bool {
	if len(kinds) == 0 {
		return nil
	}
	set := make(map[Kind]bool, len(kinds))
	for _, k := range kinds {
		set[k] = true
	}
	return set
}
