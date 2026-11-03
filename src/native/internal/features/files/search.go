package files

import (
	"context"
	"iter"
)

// scanChunk is how many entries a search scores between two checks of its context and two batches.
const scanChunk = 4096

// searchSnapshot streams the results of q over snap. Batches are yielded after each chunk that changed the best
// results, and a final batch closes the search. Cancelling ctx stops the search between chunks and ends the sequence
// without a final batch. The sequence reads one snapshot, so a concurrent index update never mixes into it.
func searchSnapshot(ctx context.Context, snap *Snapshot, q Query, env searchEnv) iter.Seq[Batch] {
	return func(yield func(Batch) bool) {
		p := newPlan(snap, q, env)
		top := newTopK(p.limit)
		better := p.better
		n := len(snap.entries)
		for start := 0; start < n; start += scanChunk {
			if ctx.Err() != nil {
				return
			}
			changed := false
			for i := start; i < min(start+scanChunk, n); i++ {
				score, ok := p.match(i)
				if ok && top.offer(hit{idx: int32(i), score: int32(score)}, better) {
					changed = true
				}
			}
			if changed && !yield(p.batch(top.sorted(better), false)) {
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		yield(p.batch(top.sorted(better), true))
	}
}

// better reports whether hit a ranks ahead of hit b: a higher name score first, then the query's sort key, then the
// name, then the entry index so the order is total.
func (p *plan) better(a, b hit) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	entryA, entryB := p.snap.entries[a.idx], p.snap.entries[b.idx]
	if c := p.compareSortKey(entryA, entryB); c != 0 {
		return c < 0
	}
	if c := compareNamesFold(p.snap.nameOf(entryA), p.snap.nameOf(entryB)); c != 0 {
		return c < 0
	}
	return a.idx < b.idx
}

// compareSortKey orders two entries by the query's sort key in the query's direction. It returns a negative number
// when a ranks first, and zero when the key does not separate them.
func (p *plan) compareSortKey(a, b entry) int {
	switch p.query.SortBy {
	case SortName:
		return p.directed(compareNamesFold(p.snap.nameOf(a), p.snap.nameOf(b)))
	case SortSize:
		return p.compareNumbers(a.size, b.size)
	case SortRecent, SortModified:
		return p.compareNumbers(a.modMs, b.modMs)
	case SortAdded, SortCreated:
		return p.compareNumbers(a.createdMs, b.createdMs)
	default:
		return 0
	}
}

// compareNumbers orders two numbers in the query's direction.
func (p *plan) compareNumbers(a, b int64) int {
	switch {
	case a == b:
		return 0
	case (a < b) == (p.query.Order == OrderAsc):
		return -1
	default:
		return 1
	}
}

// directed flips a comparison result for descending order.
func (p *plan) directed(c int) int {
	if p.query.Order == OrderDesc {
		return -c
	}
	return c
}

// batch converts the ranked hits into results.
func (p *plan) batch(hits []hit, final bool) Batch {
	results := make([]Result, 0, len(hits))
	for _, h := range hits {
		results = append(results, p.result(h))
	}
	return Batch{Results: results, Final: final}
}

// result builds the public record for one hit. It is the only place a search allocates per result.
func (p *plan) result(h hit) Result {
	e := p.snap.entries[h.idx]
	path := p.snap.Path(int(h.idx))
	return Result{
		Path:        path,
		DisplayPath: DisplayPath(p.env.home, path),
		Name:        string(p.snap.nameOf(e)),
		Extension:   p.snap.exts[e.ext],
		Kind:        e.kind,
		Size:        e.size,
		Modified:    millisToTime(e.modMs),
		Created:     millisToTime(e.createdMs),
		Score:       int(h.score),
	}
}

// compareNamesFold orders two names ASCII case-insensitively, then by length. It returns -1, 0 or 1.
func compareNamesFold(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		x, y := foldASCII(a[i]), foldASCII(b[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
