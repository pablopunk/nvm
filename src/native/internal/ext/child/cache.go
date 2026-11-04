package child

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Process-cache limits, matching the TypeScript host: at most 1000 entries, and a lifetime of at most a day.
const (
	maxCacheEntries = 1000
	maxCacheTTL     = 24 * time.Hour
)

// processCache is Cache: values live in this child's memory and are gone when the child restarts.
type processCache struct {
	mu      sync.Mutex
	clock   platform.Clock
	entries map[string]cacheEntry
}

// cacheEntry records the value, when it was set, and when it expires. A zero expiresAt never expires.
type cacheEntry struct {
	value     any
	setAt     time.Time
	expiresAt time.Time
}

func newProcessCache(clock platform.Clock) *processCache {
	return &processCache{clock: clock, entries: map[string]cacheEntry{}}
}

func (c *processCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || c.expired(entry) {
		return nil, false
	}
	return entry.value, true
}

// Stale returns the value even after it expired, until it is evicted or invalidated.
func (c *processCache) Stale(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	return entry.value, ok
}

func (c *processCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock.Now()
	entry := cacheEntry{value: value, setAt: now}
	if ttl > 0 {
		entry.expiresAt = now.Add(min(ttl, maxCacheTTL))
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= maxCacheEntries {
		c.evictOldestLocked()
	}
	c.entries[key] = entry
}

func (c *processCache) Has(key string) bool {
	_, ok := c.Get(key)
	return ok
}

// Invalidate drops one key, or every key when key is empty.
func (c *processCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if key == "" {
		c.entries = map[string]cacheEntry{}
		return
	}
	delete(c.entries, key)
}

func (c *processCache) expired(entry cacheEntry) bool {
	return !entry.expiresAt.IsZero() && !c.clock.Now().Before(entry.expiresAt)
}

// evictOldestLocked drops the entry that was set longest ago, to make room for a new key.
func (c *processCache) evictOldestLocked() {
	var oldest string
	var oldestAt time.Time
	first := true
	for key, entry := range c.entries {
		if first || entry.setAt.Before(oldestAt) {
			oldest, oldestAt, first = key, entry.setAt, false
		}
	}
	delete(c.entries, oldest)
}
