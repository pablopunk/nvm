package search

import (
	"slices"
	"strings"
)

// RankScored returns the items that match query, best first. An item's score is the best Score of the texts
// texts gives it, and an item below PhraseScoreFloor does not match, so scattered letters never show. Items of
// equal score keep their order. An exact title scores 100 and so leads every other match. An empty query returns
// the items in their order.
func RankScored[T any](items []T, query string, texts func(T) []string) []T {
	q := strings.TrimSpace(query)
	if q == "" {
		return items
	}
	type scored struct {
		item  T
		score int
	}
	matches := make([]scored, 0, len(items))
	for _, item := range items {
		best := 0
		for _, text := range texts(item) {
			best = max(best, Score(text, q))
		}
		if best >= PhraseScoreFloor {
			matches = append(matches, scored{item: item, score: best})
		}
	}
	slices.SortStableFunc(matches, func(a, b scored) int { return b.score - a.score })
	ranked := make([]T, len(matches))
	for i, match := range matches {
		ranked[i] = match.item
	}
	return ranked
}

// ItemTexts are the texts a row is matched by: its title and its id, when it has one.
func ItemTexts(item Item) []string {
	return []string{item.Title, item.ID}
}
