package search

import (
	"cmp"
	"strings"
)

// compareRows orders the ranked rows of a result: the rows of a tiered search by compareTiered, the others by
// compareScored. Two query fallbacks are compared by score, as they always were.
func compareRows(a, b Item) int {
	if a.ranked.tier == TierNone || b.ranked.tier == TierNone || a.ranked.tier == TierFallback && b.ranked.tier == TierFallback {
		return compareScored(a, b)
	}
	return compareTiered(a, b)
}

// compareScored orders rows as 1.x's compareRankedActions does (electron/search/action-ranking.ts:113-119): score, then
// tie priority, then last use, all highest first, then title ascending.
func compareScored(a, b Item) int {
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
	if c, ascii := compareASCIIFolded(a, b); ascii {
		if c != 0 {
			return c
		}
		return strings.Compare(b, a)
	}
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(b, a)
}

// compareASCIIFolded compares two ASCII strings without case, with no allocation. ascii is false when either holds
// another byte, which the caller compares as lowercased text.
func compareASCIIFolded(a, b string) (c int, ascii bool) {
	if !isASCII(a) || !isASCII(b) {
		return 0, false
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if x, y := lowerASCII(a[i]), lowerASCII(b[i]); x != y {
			return int(x) - int(y), true
		}
	}
	return len(a) - len(b), true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}
