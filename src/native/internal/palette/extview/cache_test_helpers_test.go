package extview

import "strings"

func testCachePut(cache Cache, key string, entry CacheEntry) {
	id, name, _ := strings.Cut(key, "/")
	lease := cache.acquire(id, nil)
	lease.put(name, lease.version(), entry)
	lease.release()
}

func testCacheGet(cache Cache, key string) (CacheEntry, bool) {
	id, name, _ := strings.Cut(key, "/")
	lease := cache.acquire(id, nil)
	entry, _, ok := lease.get(name)
	lease.release()
	return entry, ok
}

func (c *MemoryCache) Put(key string, entry CacheEntry)  { testCachePut(c, key, entry) }
func (c *MemoryCache) Get(key string) (CacheEntry, bool) { return testCacheGet(c, key) }
