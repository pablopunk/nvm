package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const (
	dataCacheFile       = "data-cache.json"
	maxDataCacheEntries = 64
	defaultDataTTL      = time.Minute
	defaultDataStaleTTL = 5 * time.Minute
)

// dataCacheDoc keeps the last items of each view source that asked for a cache.
type dataCacheDoc struct {
	Entries map[string]cachedItems `json:"entries"`
}

// cachedItems is one saved answer and the time it was saved, in Unix milliseconds.
type cachedItems struct {
	SavedAtMs int64           `json:"savedAtMs"`
	Items     json.RawMessage `json:"items"`
}

func newDataCache() dataCacheDoc {
	return dataCacheDoc{Entries: map[string]cachedItems{}}
}

// LoadSource resolves a view's data source with the loader the extension provides. A source without a cache key
// runs the loader every time. With one, items saved within TTL are returned without a load; past TTL the loader
// runs, and when it fails the items saved within TTL plus StaleTTL are returned instead. The Retry flag is the
// view's business, not the cache's.
func (h *Handler) LoadSource(ctx context.Context, source api.DataSource, load func(context.Context) ([]api.Item, error)) ([]api.Item, error) {
	if source.CacheKey == "" {
		return load(ctx)
	}
	docs, err := h.openedDocuments()
	if err != nil {
		return load(ctx)
	}
	saved, found := cachedEntry(docs, source.CacheKey)
	age := h.deps.Clock.Now().Sub(time.UnixMilli(saved.SavedAtMs))
	ttl := durationOrDefault(source.TTLMs, defaultDataTTL)
	if found && age < ttl {
		return decodeItems(saved.Items)
	}
	items, loadErr := load(ctx)
	if loadErr == nil {
		h.saveItems(docs, source.CacheKey, items)
		return items, nil
	}
	staleTTL := durationOrDefault(source.StaleTTLMs, defaultDataStaleTTL)
	if found && age < ttl+staleTTL {
		return decodeItems(saved.Items)
	}
	return nil, loadErr
}

func cachedEntry(docs *documents, key string) (cachedItems, bool) {
	doc, err := docs.data.Load()
	if err != nil {
		return cachedItems{}, false
	}
	entry, ok := doc.Entries[key]
	return entry, ok
}

func (h *Handler) saveItems(docs *documents, key string, items []api.Item) {
	raw, err := json.Marshal(items)
	if err != nil {
		return
	}
	now := h.deps.Clock.Now().UnixMilli()
	_ = docs.data.Update(func(doc *dataCacheDoc) {
		doc.Entries[key] = cachedItems{SavedAtMs: now, Items: raw}
		evictOldestEntries(doc)
	})
}

// evictOldestEntries keeps the cache within its entry bound by dropping the entries saved longest ago.
func evictOldestEntries(doc *dataCacheDoc) {
	for len(doc.Entries) > maxDataCacheEntries {
		var oldestKey string
		var oldestAt int64
		first := true
		for key, entry := range doc.Entries {
			if first || entry.SavedAtMs < oldestAt {
				oldestKey, oldestAt, first = key, entry.SavedAtMs, false
			}
		}
		delete(doc.Entries, oldestKey)
	}
}

func decodeItems(raw json.RawMessage) ([]api.Item, error) {
	var items []api.Item
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, unavailable("the saved items cannot be read")
	}
	return items, nil
}

func durationOrDefault(ms int64, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}
