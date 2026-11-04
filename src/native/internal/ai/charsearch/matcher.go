package charsearch

import (
	"context"
	"sync"
)

// allCategory is the category that searches every group, as the picker's "All Characters" does.
const allCategory = "all"

// The text lengths the model is asked about, in characters: shorter text matches by name alone, and longer text is cut.
const (
	minQueryLength = 3
	maxQueryLength = 80
)

// cacheLimit is how many answered searches are remembered, so a repeated search is answered at once.
const cacheLimit = 32

// Matcher answers the meaning of a search text with the decision model over one catalog. It remembers its answers,
// so the same category and text never ask twice.
type Matcher struct {
	groups  []string
	byGroup map[string][]Record

	mu    sync.Mutex
	cache map[string][]Record
	order []string
}

// NewMatcher returns a matcher over the catalog, in the order the records are given.
func NewMatcher(records []Record) *Matcher {
	groups, byGroup := semanticGroupsOf(records)
	return &Matcher{groups: groups, byGroup: byGroup, cache: map[string][]Record{}}
}

// Matches returns the characters the model finds a match for the text in category, best first. Text shorter than
// minQueryLength asks nothing. A failed decision returns its error and remembers nothing.
func (m *Matcher) Matches(ctx context.Context, decider Decider, category, text string) ([]Record, error) {
	query := normalizeQuery(text)
	if len([]rune(query)) < minQueryLength {
		return nil, nil
	}
	key := category + "\x00" + query
	if matches, ok := m.remembered(key); ok {
		return matches, nil
	}
	matches, err := m.answer(ctx, decider, query, category)
	if err != nil {
		return nil, err
	}
	m.remember(key, matches)
	return matches, nil
}

// answer runs the three steps: the likely groups, the shortlist from their characters, and the verified matches.
func (m *Matcher) answer(ctx context.Context, decider Decider, query, category string) ([]Record, error) {
	groups, err := likelyGroups(ctx, decider, query, category, m.groups)
	if err != nil {
		return nil, err
	}
	pool := recallPool(groups, m.byGroup)
	if len(pool) == 0 {
		return nil, nil
	}
	candidates, err := shortlist(ctx, decider, query, pool)
	if err != nil {
		return nil, err
	}
	return verify(ctx, decider, query, candidates)
}

// remembered returns the answer for key, if it is still in the cache.
func (m *Matcher) remembered(key string) ([]Record, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	matches, ok := m.cache[key]
	return matches, ok
}

// remember stores an answer and forgets the oldest one past the cache limit.
func (m *Matcher) remember(key string, matches []Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cache[key]; !ok {
		m.order = append(m.order, key)
	}
	m.cache[key] = matches
	for len(m.order) > cacheLimit {
		delete(m.cache, m.order[0])
		m.order = m.order[1:]
	}
}
