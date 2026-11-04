package search

import (
	"slices"
)

// mergeAppended combines the ranked rows of the providers with the appended rows. The ranked rows come first, best
// first (compareRows), and the appended rows follow in the order they were emitted, each once. An appended row that
// repeats a ranked row is dropped. The appended rows take their room out of the limit before the ranked rows do, so a
// long ranked list cannot hide the second batch. A flat result keeps every row in score order with no section labels.
// settled is false while some provider is still running; see withoutQueryFallbacks.
func mergeAppended(ranked, appended [][]Item, limit int, searching, flat, settled bool) []Item {
	rankedItems, extraItems := withoutQueryFallbacks(dedupe(flatten(ranked)), dedupe(flatten(appended)), settled)
	slices.SortStableFunc(rankedItems, compareRows)
	extra := withoutRows(extraItems, rankedItems)
	if limit > 0 {
		extra = firstN(extra, limit)
		rankedItems = firstN(rankedItems, max(0, limit-len(extra)))
	}
	out := labelRows(rankedItems, searching, flat)
	for _, it := range extra {
		if !flat {
			it.Section = it.Kind
		}
		out = append(out, it)
	}
	return out
}

// labelRows sections the ranked rows unless the result is flat.
func labelRows(items []Item, searching, flat bool) []Item {
	if flat {
		return items
	}
	return sectioned(items, searching)
}

// flatten concatenates the batches in order.
func flatten(batches [][]Item) []Item {
	var all []Item
	for _, items := range batches {
		all = append(all, items...)
	}
	return all
}

// firstN returns the first n items, or all of them when there are fewer.
func firstN(items []Item, n int) []Item {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// withoutRows drops the items that carry the ID and Kind of a row in rows. Rows without an ID are never dropped.
func withoutRows(items, rows []Item) []Item {
	taken := make(map[dedupeKey]bool, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			taken[dedupeKey{row.ID, row.Kind}] = true
		}
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.ID == "" || !taken[dedupeKey{it.ID, it.Kind}] {
			out = append(out, it)
		}
	}
	return out
}
