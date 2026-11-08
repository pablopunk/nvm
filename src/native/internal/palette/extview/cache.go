package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"time"
)

// CacheEntry is the last items a data source loaded, and when it loaded them.
type CacheEntry struct {
	Items []api.Item
	At    time.Time
}

// Cache owns installation-scoped source snapshots and their invalidation epochs.
type Cache interface {
	acquire(id string, identity any) *sourceLease
	invalidate(id string, identity any)
}

type scopeIdentity struct {
	extID    string
	identity any
}

// MemoryCache bounds source snapshots while retaining epochs for live sessions.
type MemoryCache struct{ *sourceCoordinator }

// NewMemoryCache returns a bounded, process-local cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{sourceCoordinator: newSourceCoordinator(512, 16<<20)}
}

// Invalidate invalidates data belonging to the originating installation.
func (c *MemoryCache) Invalidate(id string, identity any) { c.invalidate(id, identity) }
