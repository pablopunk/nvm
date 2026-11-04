package app

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// asyncCache holds values that cost too much to compute on the UI thread: decoded icons, previews and thumbnails.
// Get never blocks. A miss queues the load on a bounded worker pool and returns at once; when the load finishes the
// value is stored and notify runs, so the owner redraws and the next frame finds it. A full queue drops the request,
// and the next frame asks again. The store is bounded: past max entries the oldest are forgotten.
type asyncCache[V any] struct {
	mu      sync.Mutex
	values  map[string]V
	order   []string
	pending map[string]bool
	max     int
	jobs    chan asyncJob[V]
	notify  func()
	once    sync.Once
	ctx     context.Context
	workers int
	log     *logx.Logger
}

// asyncJob is one queued load.
type asyncJob[V any] struct {
	key  string
	load func() V
}

// newAsyncCache returns a cache with the given worker count, queue length and entry bound. The workers start on the
// first miss and stop when ctx ends. notify may be nil.
func newAsyncCache[V any](ctx context.Context, workers, queue, max int, notify func()) *asyncCache[V] {
	return &asyncCache[V]{
		values:  map[string]V{},
		pending: map[string]bool{},
		max:     max,
		jobs:    make(chan asyncJob[V], queue),
		notify:  notify,
		ctx:     ctx,
		workers: workers,
		log:     logx.Default(),
	}
}

// Get returns the stored value for key. On a miss it queues load and reports false; the caller draws a placeholder.
func (c *asyncCache[V]) Get(key string, load func() V) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.values[key]; ok {
		return v, true
	}
	var zero V
	if c.pending[key] {
		return zero, false
	}
	select {
	case c.jobs <- asyncJob[V]{key: key, load: load}:
		c.pending[key] = true
		c.once.Do(c.startWorkers)
	default:
	}
	return zero, false
}

// Put stores a value directly, without a load. Fixtures use it to show a cached icon.
func (c *asyncCache[V]) Put(key string, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storeLocked(key, v)
}

// Len returns how many values are stored.
func (c *asyncCache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.values)
}

// startWorkers runs the pool. Each worker loads one job at a time and stops with the cache's context.
func (c *asyncCache[V]) startWorkers() {
	for range c.workers {
		safego.Go("files.cache", func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-c.jobs:
					c.finish(job)
				}
			}
		}, safego.WithContext(c.ctx))
	}
}

// finish runs one load and stores its value. A load that panics leaves the key unstored, so it is asked for again later
// rather than marked forever pending.
func (c *asyncCache[V]) finish(job asyncJob[V]) {
	v, ok := c.loadGuarded(job)
	c.mu.Lock()
	delete(c.pending, job.key)
	if ok {
		c.storeLocked(job.key, v)
	}
	c.mu.Unlock()
	if ok && c.notify != nil {
		c.notify()
	}
}

// loadGuarded calls the job's load and reports a panic as a failed load.
func (c *asyncCache[V]) loadGuarded(job asyncJob[V]) (v V, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Warn("files.cache.load-panicked", logx.F{"reason": "panic"})
			ok = false
		}
	}()
	return job.load(), true
}

// storeLocked stores v under key and forgets the oldest entries past the bound. The caller holds mu.
func (c *asyncCache[V]) storeLocked(key string, v V) {
	if _, exists := c.values[key]; !exists {
		c.order = append(c.order, key)
	}
	c.values[key] = v
	for len(c.values) > c.max && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.values, oldest)
	}
}
