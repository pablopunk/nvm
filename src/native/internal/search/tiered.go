package search

import (
	"cmp"
	"unicode/utf8"
)

// A tiered search ranks a typed query by the match tier of each row, then by what the row is worth inside its tier:
//
//  1. the tier, best first (tiers.go). Nothing outranks it: a lower tier never passes a higher one;
//  2. inside a tier, a row the user chose for this query, or for one this query extends, when its name matches at
//     TierExact to TierSubstring;
//  3. the rank: the class base plus the frecency (frecency.go);
//  4. the shorter title, then the title in alphabetical order, then the ID.
//
// The query fallbacks have the last tier and keep the order of Score, which the learner raises as it always has (compareRows).

// rankKey is what a tiered search works out for a row once, so comparing two rows is a few integer comparisons.
type rankKey struct {
	rank     int32
	titleLen int32
	tier     Tier
	chosen   Choice
}

// rankTiered works out the tier, rank and learned choice of every row of one batch for the query the matcher holds.
func rankTiered(items []Item, m *queryMatcher, query string, learner Learner, nowMs int64) {
	for i := range items {
		it := &items[i]
		it.ranked = rankOf(it, m, query, learner, nowMs)
		if learner != nil && it.LearnKey != "" {
			it.LastUsed = max(it.LastUsed, learner.LastUsed(it.LearnKey))
		}
	}
}

func rankOf(it *Item, m *queryMatcher, query string, learner Learner, nowMs int64) rankKey {
	key := rankKey{titleLen: int32(utf8.RuneCountInString(it.Title))}
	if it.QueryFallback {
		key.tier = TierFallback
		return key
	}
	if it.Direct || it.Answers {
		key.tier, key.rank = TierExact, int32(DirectBase+it.Score)
		return key
	}
	key.tier = TierContent
	if !it.Content {
		key.tier = m.tierOf(it.Title, it.Names)
	}
	rank := classBase(classOf(it))
	if learner != nil && it.LearnKey != "" {
		use := learner.Usage(it.LearnKey, query)
		rank += Frecency(use.Count, use.LastUsed, nowMs)
		if key.tier <= TierSubstring {
			key.chosen = use.Choice
		}
	}
	key.rank = int32(rank)
	return key
}

// compareTiered orders two rows ranked by rankTiered, best first.
func compareTiered(a, b Item) int {
	return cmp.Or(
		cmp.Compare(a.ranked.tier, b.ranked.tier),
		cmp.Compare(b.ranked.chosen, a.ranked.chosen),
		cmp.Compare(b.ranked.rank, a.ranked.rank),
		cmp.Compare(a.ranked.titleLen, b.ranked.titleLen),
		compareTitles(a.Title, b.Title),
		cmp.Compare(a.ID, b.ID),
	)
}

// MatchTier is the tier a tiered search gave the row for its query, or TierNone for a row of an untiered result.
func (i Item) MatchTier() Tier { return i.ranked.tier }
