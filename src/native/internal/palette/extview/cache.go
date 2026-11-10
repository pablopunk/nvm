package extview

import (
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// CacheEntry is the last items a data source loaded, and when it loaded them.
type CacheEntry struct {
	Items           []api.Item
	PictureEvidence *api.SnapshotEvidence
	PictureIdentity any
	PictureLoader   string
	PictureVersion  *byte
	At              time.Time
}

// Cache keeps data-source snapshots so a view shows the last items at once and refreshes them in place.
type Cache interface {
	Get(key string) (CacheEntry, bool)
	Put(key string, entry CacheEntry)
}

// MemoryCache keeps snapshots for the life of the process. A restart starts empty.
type MemoryCache struct {
	mu      sync.Mutex
	entries map[string]CacheEntry
}

// NewMemoryCache returns an empty in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: map[string]CacheEntry{}}
}

// Get returns the snapshot stored under key.
func (c *MemoryCache) Get(key string) (CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	return entry, ok
}

// InvalidateExtension drops every snapshot an extension's views stored, and only those: keys are the extension id, a slash
// and the source's cache key, so an extension whose id is a prefix of another's keeps its entries.
func (c *MemoryCache) InvalidateExtension(extensionID string) {
	prefix := extensionID + "/"
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
}

// Put stores a snapshot under key, replacing the previous one.
func (c *MemoryCache) Put(key string, entry CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
}
