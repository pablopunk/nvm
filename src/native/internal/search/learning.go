package search

// Learner is the usage memory of the root. A row that carries a LearnKey is ranked by what the user chose
// for that key before, and the palette records the key when the row's primary action runs. The coordinator
// reads Boost when it merges a provider's rows; the usage package implements the interface.
type Learner interface {
	// Boost returns the learned boost of key for query, in the 1.x action-ranking units: a tenth of a score
	// point, since 1.x scores on a scale ten times this package's bands.
	Boost(key, query string) int
	// Usage returns what the learner knows of key for query: how many times it ran, when it last ran, and whether the
	// user chose it for this query or for one this query extends. It is the zero Use for a key never recorded.
	Usage(key, query string) Use
	// Record notes that the row with key ran as its primary action for query.
	Record(key, query string)
	// LastUsed returns the Unix milliseconds of key's last recorded use, or zero when it was never used.
	LastUsed(key string) int64
}

// Choice says how a row was chosen before for a query.
type Choice uint8

const (
	// ChoiceNone: the row was never chosen for the query or for a shorter one it extends.
	ChoiceNone Choice = iota
	// ChoiceExtends: the row was chosen for a query that this query extends: the user typed "lo", chose it, and now types "loo".
	ChoiceExtends
	// ChoiceExact: the row was chosen for exactly this query.
	ChoiceExact
)

// Use is the usage of one row: the count and last time of its uses, and its learned choice for a query.
type Use struct {
	Count    int
	LastUsed int64 // Unix milliseconds
	Choice   Choice
}

// boostUnitsPerPoint converts a learned boost to score points.
const boostUnitsPerPoint = 10

// bands is the ladder ScoreFuzzy scores on, lowest first. Every band is at least five points above the
// one below it, and the top band is an exact match.
var bands = []int{10, 20, 50, 60, 70, 80, 90, 95, 100}

// learned uses 1.x frecency units directly for empty-root suggestions and capped match bands for typed searches. A tiered
// search ranks typed queries by tier instead (rankedTiers).
// The coordinator applies it once per batch so a row is never boosted twice.
func (c *Coordinator) learned(items []Item, query string) []Item {
	if query != "" && c.opts.Tiered {
		return c.rankedTiers(items, query)
	}
	if c.opts.Learner == nil {
		return items
	}
	for i := range items {
		if key := items[i].LearnKey; key != "" {
			boost := c.opts.Learner.Boost(key, query)
			if query == "" {
				items[i].Score += max(0, boost)
			} else {
				items[i].Score = Boosted(items[i].Score, boost)
			}
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

// rankedTiers ranks a batch of a tiered search, for the typed query. The query fallbacks keep their learned score, so the
// order among them is the one they always had.
func (c *Coordinator) rankedTiers(items []Item, query string) []Item {
	if c.matcherQuery != query {
		c.matcher, c.matcherQuery = newQueryMatcher(query), query
	}
	if c.matcher == nil {
		return items
	}
	rankTiered(items, c.matcher, query, c.opts.Learner, c.opts.Clock.Now().UnixMilli())
	if c.opts.Learner == nil {
		return items
	}
	for i := range items {
		if key := items[i].LearnKey; key != "" && items[i].QueryFallback {
			items[i].Score = Boosted(items[i].Score, c.opts.Learner.Boost(key, query))
		}
	}
	return items
}
