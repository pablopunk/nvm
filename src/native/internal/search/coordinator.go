package search

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// The budgets of a provider call (PLAN.md B8) and the time a query has to produce its first snapshot.
const (
	// SoftBudget is how long a provider call may run before it is logged at debug level.
	SoftBudget = 20 * time.Millisecond
	// HardBudget is how long a provider call may run before its context is cancelled and it is marked slow.
	HardBudget = 100 * time.Millisecond
	// FirstSnapshot is the longest a query waits for its first snapshot: one frame. When no provider has
	// answered by then, the snapshot is empty and not settled, and the rows arrive as the providers finish.
	FirstSnapshot = 8 * time.Millisecond

	defaultCacheSize = 32
)

// Snapshot is the merged result of one query at one moment. Settled is set once every provider has
// finished or been cut off; until then more snapshots of the same generation may follow.
type Snapshot struct {
	Gen     int
	Query   string
	Items   []Item
	Settled bool
}

// Options configures a Coordinator.
type Options struct {
	// Clock measures the budgets and times the first snapshot. Tests pass a fake clock.
	Clock platform.Clock
	// Synchronous runs the providers in the calling goroutine, so the complete result is returned by Query.
	// It is for tests and goldens; the palette runs asynchronously.
	Synchronous bool
	// Limit is how many rows a merged result keeps. Zero means DefaultLimit.
	Limit int
	// CacheSize is how many queries keep their rows for instant redisplay. Zero means 32.
	CacheSize int
	// OnSnapshot receives every snapshot of an asynchronous query. It runs on a provider's goroutine, never
	// under the coordinator's lock, so it may hand the snapshot to the UI thread.
	OnSnapshot func(Snapshot)
	// Learner raises the score of the rows that carry a LearnKey by what the user chose for them. Nil means
	// no row is learned.
	Learner Learner
}

// ProviderStats counts the calls of one provider over the coordinator's life.
type ProviderStats struct {
	ID    string
	Calls int
	Slow  int           // calls cut off at the hard budget
	Last  time.Duration // duration of the most recent call that finished
}

// Coordinator runs the providers of a query concurrently, merges what they return, and keeps the merged
// result of each query for instant redisplay. A new query cancels the previous one: rows that arrive for an
// older generation are dropped, so a slow provider never overwrites a newer query's rows.
type Coordinator struct {
	opts Options

	mu      sync.Mutex
	gen     int
	query   string
	cancel  context.CancelFunc // cancels the providers of the current generation
	slots   []*slot            // one per provider of the current generation, in provider order
	pending int                // slots not finished yet
	emitted bool               // a snapshot of the current generation has been produced
	cache   *queryCache
	stats   map[string]*ProviderStats
}

// slot is one provider's part of the current generation.
type slot struct {
	id       string
	items    []Item
	started  time.Time
	finished bool
	slow     bool
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewCoordinator returns a coordinator with no queries. It panics without a clock: budgets cannot be
// measured otherwise.
func NewCoordinator(opts Options) *Coordinator {
	if opts.Clock == nil {
		panic("search: NewCoordinator needs a clock")
	}
	if opts.CacheSize == 0 {
		opts.CacheSize = defaultCacheSize
	}
	return &Coordinator{opts: opts, cache: newQueryCache(opts.CacheSize), stats: map[string]*ProviderStats{}}
}

// Query starts the search for q with the given providers and returns the rows ready at once: the rows
// remembered for q (ok), or, in Synchronous mode, the complete result. Without a remembered result the
// caller shows nothing new until the first snapshot arrives through OnSnapshot. Query never waits for a
// provider. An empty q asks the providers for their root rows.
func (c *Coordinator) Query(q string, providers []Provider) (Snapshot, bool) {
	c.mu.Lock()
	c.resetLocked()
	gen := c.gen
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.query = q
	c.slots = make([]*slot, len(providers))
	for i, p := range providers {
		c.slots[i] = &slot{id: p.ID(), started: c.opts.Clock.Now(), done: make(chan struct{})}
	}
	c.pending = len(providers)
	c.emitted = false
	remembered, hit := c.cache.get(q)
	c.mu.Unlock()

	if c.opts.Synchronous {
		for i, p := range providers {
			c.invoke(ctx, gen, i, p, q)
		}
		return c.current(gen), true
	}
	if len(providers) == 0 {
		return c.current(gen), true
	}
	for i, p := range providers {
		c.start(ctx, gen, i, p, q)
	}
	go c.firstSnapshotTimer(gen)
	if !hit {
		return Snapshot{}, false
	}
	return Snapshot{Gen: gen, Query: q, Items: remembered.items, Settled: remembered.settled}, true
}

// Cancel stops the providers of the current query and drops their rows. It is what happens to a view when
// the palette leaves it; the remembered rows stay.
func (c *Coordinator) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetLocked()
	c.slots = nil
	c.pending = 0
	c.query = ""
}

// Invalidate forgets the remembered rows, so the next query asks the providers afresh. The palette calls it
// when the data behind a query changed (a deleted clipboard item, a renamed snippet).
func (c *Coordinator) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.clear()
}

// Gen returns the generation of the current query. A snapshot is current when its Gen equals this value.
func (c *Coordinator) Gen() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen
}

// Stats returns the counters of every provider that has been called, sorted by ID.
func (c *Coordinator) Stats() []ProviderStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ProviderStats, 0, len(c.stats))
	for _, s := range c.stats {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// resetLocked starts a new generation and cancels the previous one. The caller holds c.mu.
func (c *Coordinator) resetLocked() {
	c.gen++
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// invoke runs one provider's root or search call and delivers what it emits. It is the body of every call,
// asynchronous or not.
func (c *Coordinator) invoke(ctx context.Context, gen, idx int, p Provider, q string) {
	emit := func(b Batch) {
		b.Provider = p.ID()
		c.deliver(ctx, gen, idx, b)
	}
	if q == "" {
		emit(Batch{Items: p.Root(ctx), Final: true})
		return
	}
	p.Search(ctx, q, emit)
}

// start runs one provider on its own goroutine, supervised by safego, with the budget watch beside it.
func (c *Coordinator) start(ctx context.Context, gen, idx int, p Provider, q string) {
	pctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) {
		c.mu.Unlock()
		cancel()
		return
	}
	c.slots[idx].cancel = cancel
	c.statsLocked(p.ID()).Calls++
	c.mu.Unlock()
	safego.Go("search."+p.ID(), func(context.Context) {
		defer c.complete(gen, idx)
		c.invoke(pctx, gen, idx, p, q)
	}, safego.WithContext(pctx), safego.WithClock(c.opts.Clock))
	go c.watch(gen, idx)
}

// watch logs a call that runs past the soft budget and cuts it off at the hard one. Both timers start with the
// call, so the budgets hold however the scheduler orders them.
func (c *Coordinator) watch(gen, idx int) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	done := s.done
	c.mu.Unlock()
	soft := c.opts.Clock.After(SoftBudget)
	hard := c.opts.Clock.After(HardBudget)
	select {
	case <-done:
		return
	case <-soft:
		logx.Default().Debug("search provider over its soft budget", logx.F{"provider": s.id, "soft_ms": SoftBudget.Milliseconds()})
	}
	select {
	case <-done:
	case <-hard:
		c.cutOff(gen, idx)
	}
}

// deliver merges a batch into its provider's slot and, for an asynchronous query, publishes the new snapshot.
// Batches from an older generation, or from a call whose context ended, are dropped.
func (c *Coordinator) deliver(ctx context.Context, gen, idx int, b Batch) {
	c.mu.Lock()
	if gen != c.gen || ctx.Err() != nil || idx >= len(c.slots) || c.slots[idx].finished {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	s.items = append(s.items, c.learned(b.Items, c.query)...)
	if b.Final {
		c.finishLocked(s)
	}
	if c.opts.Synchronous {
		c.mu.Unlock()
		return
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	c.publish(snap)
}

// complete marks a provider's call as returned, whatever it did. It runs after the call, panics included.
func (c *Coordinator) complete(gen, idx int) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) || c.slots[idx].finished {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	c.finishLocked(s)
	if s.cancel != nil {
		s.cancel()
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	c.publish(snap)
}

// cutOff ends a call that ran past the hard budget: its context is cancelled, its rows so far are kept, and
// it is counted as slow. The merge does not wait for it.
func (c *Coordinator) cutOff(gen, idx int) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) || c.slots[idx].finished {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	s.slow = true
	c.statsLocked(s.id).Slow++
	c.finishLocked(s)
	if s.cancel != nil {
		s.cancel()
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	logx.Default().Warn("search provider cut off at its hard budget", logx.F{"provider": s.id, "hard_ms": HardBudget.Milliseconds()})
	c.publish(snap)
}

// finishLocked marks a slot finished and releases the goroutines waiting on it. The caller holds c.mu.
func (c *Coordinator) finishLocked(s *slot) {
	if s.finished {
		return
	}
	s.finished = true
	close(s.done)
	c.pending--
	elapsed := c.opts.Clock.Now().Sub(s.started)
	c.statsLocked(s.id).Last = elapsed
}

// snapshotLocked merges the current generation and remembers the result for its query. The caller holds c.mu.
func (c *Coordinator) snapshotLocked() Snapshot {
	batches := make([][]Item, len(c.slots))
	for i, s := range c.slots {
		batches[i] = s.items
	}
	snap := Snapshot{Gen: c.gen, Query: c.query, Items: merge(batches, c.limit(), c.query != ""), Settled: c.pending == 0}
	c.emitted = true
	c.cache.put(c.query, cachedRows{items: snap.Items, settled: snap.Settled})
	return snap
}

// current returns the merged result of gen, for a synchronous query.
func (c *Coordinator) current(gen int) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return Snapshot{Gen: gen}
	}
	return c.snapshotLocked()
}

// firstSnapshotTimer makes sure a query shows something within one frame, even when every provider is slow.
func (c *Coordinator) firstSnapshotTimer(gen int) {
	<-c.opts.Clock.After(FirstSnapshot)
	c.mu.Lock()
	if gen != c.gen || c.emitted {
		c.mu.Unlock()
		return
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	c.publish(snap)
}

// publish hands a snapshot to OnSnapshot. It is called without the lock.
func (c *Coordinator) publish(snap Snapshot) {
	if c.opts.OnSnapshot != nil {
		c.opts.OnSnapshot(snap)
	}
}

func (c *Coordinator) limit() int {
	if c.opts.Limit > 0 {
		return c.opts.Limit
	}
	return DefaultLimit
}

// statsLocked returns the counters of a provider, creating them on first use. The caller holds c.mu.
func (c *Coordinator) statsLocked(id string) *ProviderStats {
	s, ok := c.stats[id]
	if !ok {
		s = &ProviderStats{ID: id}
		c.stats[id] = s
	}
	return s
}
