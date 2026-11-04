package search

// Learner is the usage memory of the root. A row that carries a LearnKey is ranked by what the user chose
// for that key before, and the palette records the key when the row's primary action runs. The coordinator
// reads Boost when it merges a provider's rows; the usage package implements the interface.
type Learner interface {
	// Boost returns the learned boost of key for query, in the 1.x action-ranking units: a tenth of a score
	// point, since 1.x scores on a scale ten times this package's bands.
	Boost(key, query string) int
	// Record notes that the row with key ran as its primary action for query.
	Record(key, query string)
	// LastUsed returns the Unix milliseconds of key's last recorded use, or zero when it was never used.
	LastUsed(key string) int64
}

// boostUnitsPerPoint converts a learned boost to score points.
const boostUnitsPerPoint = 10

// bands is the ladder ScoreFuzzy scores on, lowest first. Every band is at least five points above the
// one below it, and the top band is an exact match.
var bands = []int{10, 20, 50, 60, 70, 80, 90, 95, 100}

// learned raises the score of each learned row of a batch for query. The coordinator calls it once per batch,
// as the batch arrives, so a row is never boosted twice.
func (c *Coordinator) learned(items []Item, query string) []Item {
	if c.opts.Learner == nil {
		return items
	}
	for i := range items {
		if key := items[i].LearnKey; key != "" {
			items[i].Score = Boosted(items[i].Score, c.opts.Learner.Boost(key, query))
			items[i].LastUsed = max(items[i].LastUsed, c.opts.Learner.LastUsed(key))
		}
	}
	return items
}

// Boosted returns score raised by a learned boost (in the units of Learner.Boost). The boost never lifts a
// row to the next band above its score, so a boosted partial match cannot pass an exact match or a better
// band; 1.x bounds its query-fallback boost the same way (min(9, boost) below its fallback band). A score
// at or above the top band takes the whole boost.
func Boosted(score, boost int) int {
	points := max(0, boost) / boostUnitsPerPoint
	if above := bandAbove(score); above > 0 {
		points = min(points, above-score-1)
	}
	return score + max(0, points)
}

// bandAbove returns the lowest band above score, or 0 when score is at or above the top band.
func bandAbove(score int) int {
	for _, band := range bands {
		if band > score {
			return band
		}
	}
	return 0
}
