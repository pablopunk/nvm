package search

type cachedItemKey struct{ id, kind, title, learnKey string }

func cacheKey(item Item) cachedItemKey {
	if item.ID != "" {
		return cachedItemKey{id: item.ID, kind: item.Kind}
	}
	return cachedItemKey{kind: item.Kind, title: item.Title, learnKey: item.LearnKey}
}

// providerCache keeps the visible snapshot by source. Large inventories must not multiply across remembered queries.
func (c *Coordinator) providerCache(visible []Item) map[string][]Item {
	keys := make(map[cachedItemKey]bool, len(visible))
	for _, item := range visible {
		keys[cacheKey(item)] = true
	}
	rows := make(map[string][]Item, len(c.slots))
	for _, s := range c.slots {
		for _, item := range s.items {
			if keys[cacheKey(item)] {
				rows[s.id] = append(rows[s.id], item)
			}
			if len(rows[s.id]) >= c.limit() {
				break
			}
		}
	}
	return rows
}
