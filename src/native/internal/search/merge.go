package search

import (
	"cmp"
	"slices"
)

// DefaultLimit is how many rows a merged result keeps: the first rows by score, as the prototype kept.
const DefaultLimit = 40

// merge combines the rows of several providers into one result: rows with an ID are de-duplicated by ID
// and Kind (the first provider wins), query fallbacks are held or dropped, the rows are sorted by score, highest
// first, with ties in provider order, the result is cut to limit, and the rows are sectioned. searching tells
// sectioning whether the query is typed, which changes the top hit and the grouping of commands. settled is false
// while some provider is still running.
func merge(batches [][]Item, limit int, searching, settled bool) []Item {
	var all []Item
	for _, items := range batches {
		all = append(all, items...)
	}
	all = withoutQueryFallbacks(dedupe(all), settled)
	slices.SortStableFunc(all, func(a, b Item) int { return cmp.Compare(b.Score, a.Score) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return sectioned(all, searching)
}

// withoutQueryFallbacks drops the query fallbacks an answer makes unnecessary. While a provider is still running, it
// drops every fallback too: a fallback shown early would vanish when the answer it loses to arrives a moment later.
func withoutQueryFallbacks(items []Item, settled bool) []Item {
	answered := slices.ContainsFunc(items, func(it Item) bool { return it.Answers })
	if answered || !settled {
		return slices.DeleteFunc(items, func(it Item) bool { return it.QueryFallback })
	}
	return items
}

// dedupeKey identifies a row for de-duplication.
type dedupeKey struct{ id, kind string }

// dedupe keeps the first row of each ID and Kind. Rows without an ID are never merged: two snippets or
// two quicklinks may share a Kind and have no ID of their own.
func dedupe(items []Item) []Item {
	seen := make(map[dedupeKey]bool, len(items))
	out := items[:0]
	for _, it := range items {
		if it.ID != "" {
			key := dedupeKey{it.ID, it.Kind}
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, it)
	}
	return out
}

// sectionOrder is the order of the sections a typed query shows; kinds not listed sort with the commands.
var sectionOrder = []string{KindCalculator, KindTimer, KindQuicklinks, KindSnippets, KindCommands, KindApplications, KindFiles, KindClipboard, KindConversations, KindWeb, KindSearch}

// sectionCommands is the rank of the commands section, which extension rows share.
const sectionCommands = 4

func sectionRank(kind string) int {
	if i := slices.Index(sectionOrder, kind); i >= 0 {
		return i
	}
	return sectionCommands
}

// sectioned labels runs of results like Spotlight: a "Top Hit", then groups by kind.
func sectioned(items []Item, searching bool) []Item {
	if len(items) == 0 {
		return items
	}
	var top []Item
	rest := items
	if searching {
		top, rest = items[:1], slices.Clone(items[1:])
	} else {
		rest = slices.Clone(items)
	}
	slices.SortStableFunc(rest, func(a, b Item) int { return cmp.Compare(sectionRank(a.Kind), sectionRank(b.Kind)) })
	out := make([]Item, 0, len(items))
	for _, it := range top {
		it.Section = "Top Hit"
		out = append(out, it)
	}
	for _, it := range rest {
		it.Section = it.Kind
		if !searching && it.Kind == KindCommands {
			it.Section = "Suggestions"
		}
		out = append(out, it)
	}
	return out
}
