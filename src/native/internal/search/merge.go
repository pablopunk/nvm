package search

import (
	"cmp"
	"slices"
)

// DefaultLimit is how many rows a merged result keeps: the first rows by score, as the prototype kept.
const DefaultLimit = 40

// withoutQueryFallbacks drops the query fallbacks an answer makes unnecessary. An answer is a row that marks
// AnswersQuery; the answer may come from any provider, so the ranked and appended rows are checked together. While a
// first search is still running, every fallback is dropped too. A previously settled query keeps its fallbacks
// during revalidation, unless a current or cached answer suppresses them.
func withoutQueryFallbacks(ranked, appended []Item, settled bool) ([]Item, []Item) {
	answered := slices.ContainsFunc(ranked, isAnswer) || slices.ContainsFunc(appended, isAnswer)
	if answered || !settled {
		return slices.DeleteFunc(ranked, isQueryFallback), slices.DeleteFunc(appended, isQueryFallback)
	}
	return ranked, appended
}

func isAnswer(it Item) bool { return it.Answers }

func isQueryFallback(it Item) bool { return it.QueryFallback }

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
