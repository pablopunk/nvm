// Package usage remembers which rows the user chose in the root and ranks them by it. It implements
// search.Learner. The weights and the decay are those of the 1.x action ranking (action-ranker.ts): a use
// adds six units, uses count up to ninety, and a recent use adds up to twenty units that fade by one unit an
// hour. The rows are remembered per query as well, so a row chosen for "cl" ranks higher for "cl" than for "c".
//
// The learner is safe for concurrent use. Its memory is bounded: the least recently used rows and queries go
// first. It persists to usage.json through the store, which writes debounced and atomically.
//
// Layer: search, above store and platform.
package usage

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// The 1.x weights. Units are the 1.x action ranking's, a tenth of a score point of this package's bands.
const (
	// UsageWeight is what one use adds (1.x usageBoost: count * 6).
	UsageWeight = 6
	// UsageCap bounds the usage term and the per-query term (1.x: min(90, count * 6)).
	UsageCap = 90
	// RecencyCap is what a use just now adds (1.x MAX_RECENCY_BOOST). It fades by one unit an hour and is gone
	// after RecencyCap hours.
	RecencyCap = 20
)

// The memory bounds: rows and queries the learner keeps. The least recently used go first.
const (
	MaxItems          = 500
	MaxQueriesPerItem = 16
	// maxQueryRunes bounds the length of a remembered query.
	maxQueryRunes = 64
	// schemaVersion is the version of usage.json this build writes.
	schemaVersion = 1
	// documentName is the store file the learner persists to.
	documentName = "usage.json"
)

// Learner remembers the uses of each row, and of each row for each query it was used with.
type Learner struct {
	clock platform.Clock
	log   *logx.Logger

	mu    sync.RWMutex
	items map[string]item
	doc   *store.Collection[document]
}

// item is what the learner knows of one row. LastUsed is in Unix milliseconds, so the file stays plain JSON.
type item struct {
	Count    int                 `json:"count"`
	LastUsed int64               `json:"lastUsed"`
	Queries  map[string]queryUse `json:"queries,omitempty"`
}

// queryUse is what the learner knows of one row for one query.
type queryUse struct {
	Count    int   `json:"count"`
	LastUsed int64 `json:"lastUsed"`
}

// document is the content of usage.json.
type document struct {
	Items map[string]item `json:"items"`
}

// Open returns the learner that persists to usage.json in s, loaded from the file. A corrupt file is moved
// aside by the store and the learner starts empty. The learner is usable even when the load fails; the error
// reports the failure so the caller can say so.
func Open(s *store.Store, clock platform.Clock, log *logx.Logger) (*Learner, error) {
	doc := store.Doc(s, documentName, schemaVersion, emptyDocument)
	value, err := doc.Load()
	l := &Learner{clock: clock, log: log, items: sanitize(value.Items), doc: doc}
	return l, err
}

// NewMemory returns a learner that keeps nothing on disk. Tests and goldens use it.
func NewMemory(clock platform.Clock) *Learner {
	return &Learner{clock: clock, log: logx.Default(), items: map[string]item{}}
}

func emptyDocument() document { return document{Items: map[string]item{}} }

// Record notes that the row id ran as its primary action for query. An empty id is not learned.
func (l *Learner) Record(id, query string) {
	if id == "" {
		return
	}
	key := queryKey(query)
	now := l.clock.Now().UnixMilli()
	l.mu.Lock()
	defer l.mu.Unlock()
	it := l.items[id]
	if it.Queries == nil {
		it.Queries = map[string]queryUse{}
	}
	it.Count++
	it.LastUsed = now
	use := it.Queries[key]
	use.Count++
	use.LastUsed = now
	it.Queries[key] = use
	it.Queries = pruneQueries(it.Queries)
	l.items[id] = it
	l.items = pruneItems(l.items)
	l.saveLocked()
}

// Boost returns the learned boost of row id for query, in the units of search.Learner.Boost: the usage term,
// the term of this query, and the recency term. It is zero for a row that was never used.
func (l *Learner) Boost(id, query string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	it, ok := l.items[id]
	if !ok {
		return 0
	}
	now := l.clock.Now().UnixMilli()
	usage := min(UsageCap, it.Count*UsageWeight)
	affinity := min(UsageCap, it.Queries[queryKey(query)].Count*UsageWeight)
	return usage + affinity + recencyBoost(it.LastUsed, now)
}

// Uses returns how many times row id was recorded.
func (l *Learner) Uses(id string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.items[id].Count
}

// recencyBoost is the recency term of a use at lastUsed (Unix milliseconds) seen at now: RecencyCap units
// less one per hour of age, never below zero. Zero means never used. The 1.x term keeps its fraction; this
// one is whole units, since the boosts are whole score points.
func recencyBoost(lastUsed, now int64) int {
	if lastUsed == 0 {
		return 0
	}
	age := time.Duration(now-lastUsed) * time.Millisecond
	return max(0, int(math.Floor(RecencyCap-max(0, age).Hours())))
}

// queryKey is how a query is remembered: trimmed, lowercased and bounded in length.
func queryKey(query string) string {
	key := strings.ToLower(strings.TrimSpace(query))
	if runes := []rune(key); len(runes) > maxQueryRunes {
		key = string(runes[:maxQueryRunes])
	}
	return key
}

// saveLocked writes the memory to the store, which debounces the write. The caller holds l.mu. The store
// receives a copy, so later changes to the memory never race with a write.
func (l *Learner) saveLocked() {
	if l.doc == nil {
		return
	}
	snapshot := cloneItems(l.items)
	if err := l.doc.Update(func(d *document) { d.Items = snapshot }); err != nil && l.log != nil {
		l.log.Warn("usage memory was not saved", logx.F{"error": err.Error()})
	}
}

// sanitize returns a usable memory from loaded items: entries with no uses are dropped, and the bounds hold.
func sanitize(loaded map[string]item) map[string]item {
	items := make(map[string]item, len(loaded))
	for id, it := range loaded {
		if id == "" || it.Count <= 0 {
			continue
		}
		if it.Queries == nil {
			it.Queries = map[string]queryUse{}
		}
		for key, use := range it.Queries {
			if use.Count <= 0 {
				delete(it.Queries, key)
			}
		}
		it.Queries = pruneQueries(it.Queries)
		items[id] = it
	}
	return pruneItems(items)
}

// pruneItems drops the least recently used rows until MaxItems remain.
func pruneItems(items map[string]item) map[string]item {
	if len(items) <= MaxItems {
		return items
	}
	type ranked struct {
		id       string
		lastUsed int64
	}
	order := make([]ranked, 0, len(items))
	for id, it := range items {
		order = append(order, ranked{id, it.LastUsed})
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].lastUsed != order[j].lastUsed {
			return order[i].lastUsed < order[j].lastUsed
		}
		return order[i].id < order[j].id
	})
	for _, r := range order[:len(items)-MaxItems] {
		delete(items, r.id)
	}
	return items
}

// pruneQueries drops the least recently used queries of one row until MaxQueriesPerItem remain.
func pruneQueries(queries map[string]queryUse) map[string]queryUse {
	if len(queries) <= MaxQueriesPerItem {
		return queries
	}
	type ranked struct {
		key      string
		lastUsed int64
	}
	order := make([]ranked, 0, len(queries))
	for key, use := range queries {
		order = append(order, ranked{key, use.LastUsed})
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].lastUsed != order[j].lastUsed {
			return order[i].lastUsed < order[j].lastUsed
		}
		return order[i].key < order[j].key
	})
	for _, r := range order[:len(queries)-MaxQueriesPerItem] {
		delete(queries, r.key)
	}
	return queries
}

// cloneItems returns a deep copy of the memory.
func cloneItems(items map[string]item) map[string]item {
	out := make(map[string]item, len(items))
	for id, it := range items {
		copied := it
		copied.Queries = make(map[string]queryUse, len(it.Queries))
		for key, use := range it.Queries {
			copied.Queries[key] = use
		}
		out[id] = copied
	}
	return out
}

var _ search.Learner = (*Learner)(nil)

// LastUsed returns the Unix milliseconds of row id's last recorded use, or zero when it was never used.
func (l *Learner) LastUsed(id string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.items[id].LastUsed
}
