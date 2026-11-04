package search

// queryCache remembers the last merged rows for recently typed queries, so that typing a query again shows
// its rows at once while the providers refresh them. It holds a few dozen queries and drops the least
// recently used. It is not safe for concurrent use: the coordinator guards it.
type queryCache struct {
	limit int
	order []string // least recently used first
	rows  map[string]cachedRows
}

type cachedRows struct {
	items   []Item
	settled bool
}

func newQueryCache(limit int) *queryCache {
	return &queryCache{limit: limit, rows: map[string]cachedRows{}}
}

// get returns the rows remembered for q and marks q as used.
func (c *queryCache) get(q string) (cachedRows, bool) {
	entry, ok := c.rows[q]
	if ok {
		c.touch(q)
	}
	return entry, ok
}

// put remembers the rows of q, evicting the least recently used query when the cache is full.
func (c *queryCache) put(q string, entry cachedRows) {
	if _, ok := c.rows[q]; !ok && len(c.rows) >= c.limit && c.limit > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.rows, oldest)
	}
	c.rows[q] = entry
	c.touch(q)
}

// clear forgets every query.
func (c *queryCache) clear() {
	c.order = nil
	c.rows = map[string]cachedRows{}
}

// touch moves q to the most recently used end of the order.
func (c *queryCache) touch(q string) {
	for i, k := range c.order {
		if k == q {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, q)
}
