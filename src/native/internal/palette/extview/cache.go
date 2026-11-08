package extview

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// CacheEntry is the last items a data source loaded, and when it loaded them.
type CacheEntry struct {
	Items []api.Item
	At    time.Time
}

// Cache keeps data-source snapshots so a view shows the last items at once and refreshes them in place.
type Cache interface {
	Get(key string) (CacheEntry, bool)
	Put(key string, entry CacheEntry)
	Generation(extID string) uint64
	PutGeneration(extID, key string, generation uint64, entry CacheEntry) bool
	InvalidateExtension(extID string)
	Scope(extID string, identity any) string
}

// MemoryCache keeps snapshots for the life of the process. A restart starts empty.
type MemoryCache struct {
	mu          sync.Mutex
	entries     map[string]CacheEntry
	generations map[string]uint64
	scopes      map[scopeIdentity]string
	nextScope   uint64
}

// NewMemoryCache returns an empty in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{entries: map[string]CacheEntry{}, generations: map[string]uint64{}, scopes: map[scopeIdentity]string{}}
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

// Generation identifies the current extension snapshot epoch.
func (c *MemoryCache) Generation(extID string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generations[extID]
}

// PutGeneration refuses results started before an invalidation.
func (c *MemoryCache) PutGeneration(extID, key string, generation uint64, entry CacheEntry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generations[extID] != generation {
		return false
	}
	c.entries[key] = entry
	return true
}

// InvalidateExtension discards only this extension's snapshots and fences outstanding writes.
func (c *MemoryCache) InvalidateExtension(extID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generations[extID]++
	for key := range c.entries {
		if strings.HasPrefix(key, extID+"/") {
			delete(c.entries, key)
		}
	}
}

type scopeIdentity struct {
	extID    string
	identity any
}

// Scope gives each comparable installation identity a process-local namespace.
func (c *MemoryCache) Scope(extID string, identity any) string {
	if identity == nil || !reflect.TypeOf(identity).Comparable() {
		return extID
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := scopeIdentity{extID, identity}
	scope, ok := c.scopes[key]
	if !ok {
		c.nextScope++
		scope = fmt.Sprintf("%s#%d", extID, c.nextScope)
		c.scopes[key] = scope
	}
	return scope
}
