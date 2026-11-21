package child

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// dataSources declares view sources and reads bundled datasets. A source is a description the host resolves later,
// so declaring one makes no call and needs no host round trip. A bundle is read through the host, once per process.
type dataSources struct {
	link    *hostLink
	bundles *bundleCache
}

func (d dataSources) Loader(loader string, retry bool) api.DataSource {
	return api.DataSource{Loader: loader, Retry: retry}
}

func (d dataSources) StaleWhileRevalidate(loader, cacheKey string, ttl, staleTTL time.Duration, retry bool) api.DataSource {
	return api.DataSource{
		Loader:     loader,
		CacheKey:   cacheKey,
		TTLMs:      ttl.Milliseconds(),
		StaleTTLMs: staleTTL.Milliseconds(),
		Retry:      retry,
	}
}

// Bundled returns the bundled dataset name. Its records are read on the first call and kept by the context's bundle
// cache, so every later Records call returns them without the host decoding or sending them again.
func (d dataSources) Bundled(name string) api.Bundle {
	return bundleReader{name: name, link: d.link, cache: d.bundles}
}

// bundleCache keeps the records of each bundled dataset this context has read.
type bundleCache struct {
	mu      sync.Mutex
	records map[string][]api.BundleRecord
}

func newBundleCache() *bundleCache {
	return &bundleCache{records: map[string][]api.BundleRecord{}}
}

// bundleReader is the api.Bundle of one dataset name.
type bundleReader struct {
	name  string
	link  *hostLink
	cache *bundleCache
}

func (r bundleReader) Records() ([]api.BundleRecord, error) {
	if records, ok := r.cache.lookup(r.name); ok {
		return records, nil
	}
	var records []api.BundleRecord
	if err := r.link.Call(protocol.ServiceBundle, protocol.OpRecords, protocol.BundleArgs{Name: r.name}, &records); err != nil {
		return nil, err
	}
	r.cache.store(r.name, records)
	return records, nil
}

func (c *bundleCache) lookup(name string) ([]api.BundleRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	records, ok := c.records[name]
	return records, ok
}

func (c *bundleCache) store(name string, records []api.BundleRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[name] = records
}
