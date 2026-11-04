package child

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// dataSources declares view sources. A source is a description the host resolves later, so declaring one makes no
// call and needs no host round trip.
type dataSources struct{}

func (dataSources) Loader(loader string, retry bool) api.DataSource {
	return api.DataSource{Loader: loader, Retry: retry}
}

func (dataSources) StaleWhileRevalidate(loader, cacheKey string, ttl, staleTTL time.Duration, retry bool) api.DataSource {
	return api.DataSource{
		Loader:     loader,
		CacheKey:   cacheKey,
		TTLMs:      ttl.Milliseconds(),
		StaleTTLMs: staleTTL.Milliseconds(),
		Retry:      retry,
	}
}
