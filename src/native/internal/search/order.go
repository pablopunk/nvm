package search

import (
	"cmp"
	"strings"
)

// compareRows orders the ranked rows of the root as 1.x's compareRankedActions does (src/app/electron/search/
// action-ranking.ts:113-119): score, then tie priority, then last use, all highest first, then title ascending.
func compareRows(a, b Item) int {
	return cmp.Or(
		cmp.Compare(b.Score, a.Score),
		cmp.Compare(b.TiePriority, a.TiePriority),
		cmp.Compare(b.LastUsed, a.LastUsed),
		compareTitles(a.Title, b.Title),
	)
}

// compareTitles approximates the localeCompare that 1.x sorts titles by: letters compare without case first, and at
// equal letters the lowercase one comes first. Punctuation and accents are not weighted as ICU weights them.
func compareTitles(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(b, a)
}
