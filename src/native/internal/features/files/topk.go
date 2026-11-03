package files

import "slices"

// hit is a candidate result: an entry index and its name score.
type hit struct {
	idx   int32
	score int32
}

// topK keeps the best limit hits seen so far in a heap whose root is the worst kept hit, so a candidate costs one
// comparison unless it displaces the root.
type topK struct {
	limit int
	items []hit
}

// newTopK returns an empty selection of at most limit hits.
func newTopK(limit int) *topK {
	return &topK{limit: limit, items: make([]hit, 0, min(limit, 256))}
}

// offer keeps h when it ranks among the best limit hits. better reports whether its first argument ranks ahead of its
// second. It reports whether the selection changed.
func (t *topK) offer(h hit, better func(a, b hit) bool) bool {
	if len(t.items) < t.limit {
		t.items = append(t.items, h)
		t.siftUp(len(t.items)-1, better)
		return true
	}
	if !better(h, t.items[0]) {
		return false
	}
	t.items[0] = h
	t.siftDown(0, better)
	return true
}

// siftUp restores the heap after an append at i. The parent of every hit must be no better than its children, so a
// better parent moves down.
func (t *topK) siftUp(i int, better func(a, b hit) bool) {
	for i > 0 {
		parent := (i - 1) / 2
		if !better(t.items[parent], t.items[i]) {
			return
		}
		t.items[i], t.items[parent] = t.items[parent], t.items[i]
		i = parent
	}
}

// siftDown restores the heap after the root was replaced.
func (t *topK) siftDown(i int, better func(a, b hit) bool) {
	n := len(t.items)
	for {
		worst := i
		left, right := 2*i+1, 2*i+2
		if left < n && better(t.items[worst], t.items[left]) {
			worst = left
		}
		if right < n && better(t.items[worst], t.items[right]) {
			worst = right
		}
		if worst == i {
			return
		}
		t.items[i], t.items[worst] = t.items[worst], t.items[i]
		i = worst
	}
}

// sorted returns a copy of the kept hits, best first.
func (t *topK) sorted(better func(a, b hit) bool) []hit {
	out := slices.Clone(t.items)
	slices.SortFunc(out, func(a, b hit) int {
		switch {
		case better(a, b):
			return -1
		case better(b, a):
			return 1
		default:
			return 0
		}
	})
	return out
}
