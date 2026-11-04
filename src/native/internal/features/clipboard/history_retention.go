package clipboard

import "time"

// enforceLocked drops the unpinned entries that retention no longer allows, and returns how many went. It
// applies, in order: expiry by age, the entry cap (oldest first), the image byte cap and the memory cap.
// The caller holds h.mu.
func (h *History) enforceLocked(now time.Time) int {
	p := h.policy
	drop := map[*record]bool{}
	if p.MaxAge > 0 {
		for _, r := range h.order {
			if now.Sub(r.entry.CreatedAt) > p.MaxAge {
				drop[r] = true
			}
		}
	}
	kept := 0
	for _, r := range h.order {
		if !drop[r] {
			kept++
		}
	}
	excess := kept - p.MaxEntries
	imageBytes, memory := h.imageBytes, h.memory
	for r := range drop {
		imageBytes -= r.image
		memory -= r.memory
	}
	for i := len(h.order) - 1; i >= 0; i-- {
		r := h.order[i]
		if drop[r] {
			continue
		}
		overCount := excess > 0
		overImages := imageBytes > p.MaxImageBytes && r.image > 0
		overMemory := memory > p.MaxMemoryBytes && r.memory > 0
		if !overCount && !overImages && !overMemory {
			continue
		}
		drop[r] = true
		excess--
		imageBytes -= r.image
		memory -= r.memory
	}
	return h.dropLocked(drop)
}

// dropLocked removes the records in drop from the order and the indexes, and returns how many went. The
// caller holds h.mu.
func (h *History) dropLocked(drop map[*record]bool) int {
	if len(drop) == 0 {
		return 0
	}
	kept := h.order[:0]
	removed := 0
	for _, r := range h.order {
		if drop[r] {
			h.indexLocked(r, -1)
			removed++
			continue
		}
		kept = append(kept, r)
	}
	clear(h.order[len(kept):])
	h.order = kept
	return removed
}
