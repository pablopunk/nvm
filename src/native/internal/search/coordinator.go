package search

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// The budgets of a provider call (PLAN.md B8) and the time a query has to produce its first snapshot.
const (
	// SoftBudget is how long a provider call may run before it is logged at debug level.
	SoftBudget = 20 * time.Millisecond
	// HardBudget is how long a provider call may run before it is marked slow and stops holding up the snapshot.
	// Its rows that arrive later still merge into the same query.
	HardBudget = 100 * time.Millisecond
	// FirstSnapshot is the longest a query waits for its first snapshot: one frame. When no provider has
	// answered by then, the snapshot is empty and not settled, and the rows arrive as the providers finish.
	FirstSnapshot = 8 * time.Millisecond

	defaultCacheSize = 32
)

// Snapshot is the merged result of one query at one moment. Settled is set once every provider has
// finished or been cut off; until then more snapshots of the same generation may follow. Pending is set while
// a provider reports a call in flight (Batch.Pending), so the rows shown are not the whole answer yet.
type Snapshot struct {
	revision uint64
	Gen      int
	Query    string
	Items    []Item
	Settled  bool
	Pending  bool
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
	// Flat lists the rows in score order with no section labels, as 1.x's root list does, for every query.
	Flat bool
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

	mu                sync.Mutex
	publishMu         sync.Mutex
	revision          uint64
	publishedRevision uint64
	gen               int
	query             string
	cancel            context.CancelFunc // cancels the providers of the current generation
	slots             []*slot            // one per provider of the current generation, in provider order
	pending           int                // slots not finished yet
	emitted           bool               // a snapshot of the current generation has been produced
	cache             *queryCache
	stats             map[string]*ProviderStats
}

// slot is one provider's part of the current generation.
type slot struct {
	id           string
	appended     bool
	items        []Item
	refreshing   bool
	started      time.Time
	budget       Budget
	finished     bool
	returned     bool
	selfBudgeted bool
	pending      bool
	slow         bool
	cancel       context.CancelFunc
	done         chan struct{}
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
		c.slots[i] = &slot{id: p.ID(), appended: isAppended(p), started: c.opts.Clock.Now(), budget: budgetOf(p), selfBudgeted: ownsCallBudgets(p), done: make(chan struct{}), refreshing: true}
	}
	c.pending = len(providers)
	c.emitted = false
	remembered, hit := c.cache.get(q)
	var warm Snapshot
	if hit {
		for _, s := range c.slots {
			s.items = append([]Item(nil), remembered.providers[s.id]...)
		}
		warm = Snapshot{Gen: gen, Query: q, Items: mergeCached(c.slots, c.limit(), q != "", c.opts.Flat, remembered.settled), Settled: remembered.settled}
	}
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
	return warm, true
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

// Invalidate forgets remembered rows. Use it when a snapshot must be discarded; ordinary refreshes call Query,
// which always revalidates providers while keeping cached navigation immediate.
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
		c.deliver(gen, idx, b)
	}
	answerCall(ctx, p, q, emit)
}

// answerCall runs the root call of p, streamed when it streams, or its search call for q.
func answerCall(ctx context.Context, p Provider, q string, emit func(Batch)) {
	if q != "" {
		p.Search(ctx, q, emit)
		return
	}
	if streamer, streams := p.(RootStreamer); streams {
		streamer.RootStream(ctx, emit)
		return
	}
	emit(Batch{Items: p.Root(ctx), Final: true})
}

func mergeCached(slots []*slot, limit int, searching, flat, settled bool) []Item {
	var ranked, appended [][]Item
	for _, s := range slots {
		if s.appended {
			appended = append(appended, s.items)
		} else {
			ranked = append(ranked, s.items)
		}
	}
	return mergeAppended(ranked, appended, limit, searching, flat, settled)
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
		returned := false
		defer func() { c.complete(gen, idx, returned) }()
		c.invoke(pctx, gen, idx, p, q)
		returned = true
	}, safego.WithContext(pctx), safego.WithClock(c.opts.Clock))
	go c.watch(gen, idx)
}

// snapshotLocked merges the current generation and remembers the result for its query. The caller holds c.mu.
func (c *Coordinator) snapshotLocked() Snapshot {
	c.revision++
	var ranked, appended [][]Item
	for _, s := range c.slots {
		if s.appended {
			appended = append(appended, s.items)
		} else {
			ranked = append(ranked, s.items)
		}
	}
	settled := c.pending == 0
	snap := Snapshot{revision: c.revision, Gen: c.gen, Query: c.query, Items: mergeAppended(ranked, appended, c.limit(), c.query != "", c.opts.Flat, settled), Settled: settled, Pending: c.pendingCallsLocked()}
	c.emitted = true
	providerRows := c.providerCache(snap.Items)
	c.cache.put(c.query, cachedRows{items: snap.Items, providers: providerRows, settled: snap.Settled})
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
	c.publishMu.Lock()
	defer c.publishMu.Unlock()
	c.mu.Lock()
	if snap.Gen != c.gen || snap.revision <= c.publishedRevision {
		c.mu.Unlock()
		return
	}
	c.publishedRevision = snap.revision
	c.mu.Unlock()
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

func ownsCallBudgets(p Provider) bool {
	_, ok := p.(SelfBudgetedProvider)
	return ok
}
