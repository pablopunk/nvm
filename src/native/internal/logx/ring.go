package logx

import "maps"

// RingCapacity is how many of the newest entries stay in memory for the Diagnostics view.
const RingCapacity = 1000

// ring keeps the newest entries in fixed memory, overwriting the oldest. It is not safe for concurrent
// use; the core guards it with its mutex.
type ring struct {
	items []Entry
	start int
	count int
}

func newRing(capacity int) *ring {
	return &ring{items: make([]Entry, capacity)}
}

// push stores e, dropping the oldest entry when the ring is full.
func (r *ring) push(e Entry) {
	if r.count < len(r.items) {
		r.items[(r.start+r.count)%len(r.items)] = e
		r.count++
		return
	}
	r.items[r.start] = e
	r.start = (r.start + 1) % len(r.items)
}

// newest returns up to limit entries, oldest first. Fields are copied so the caller cannot change the
// ring. A limit of zero or less returns everything retained.
func (r *ring) newest(limit int) []Entry {
	n := r.count
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]Entry, 0, n)
	for i := r.count - n; i < r.count; i++ {
		e := r.items[(r.start+i)%len(r.items)]
		e.Fields = maps.Clone(e.Fields)
		out = append(out, e)
	}
	return out
}
