package extview

import (
	"encoding/json"
	"reflect"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// sourceCoordinator owns source epochs independently of snapshot eviction.
type sourceCoordinator struct {
	mu         sync.Mutex
	records    map[scopeIdentity]*sourceRecord
	next       uint64
	entries    int
	bytes      int
	maxEntries int
	maxBytes   int
}

type sourceRecord struct {
	key       scopeIdentity
	epoch     uint64
	refs      int
	snapshots map[string]sourceSnapshot
}

type sourceSnapshot struct {
	entry CacheEntry
	bytes int
}

type sourceLease struct {
	owner    *sourceCoordinator
	record   *sourceRecord
	released bool
}

func newSourceCoordinator(maxEntries, maxBytes int) *sourceCoordinator {
	return &sourceCoordinator{records: make(map[scopeIdentity]*sourceRecord), maxEntries: maxEntries, maxBytes: maxBytes}
}

// acquire keeps epoch authority alive until release, even if all snapshots are evicted.
func (c *sourceCoordinator) acquire(id string, identity any) *sourceLease {
	if identity != nil && !reflect.TypeOf(identity).Comparable() {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := scopeIdentity{extID: id, identity: identity}
	record := c.records[key]
	if record == nil {
		c.next++
		record = &sourceRecord{key: key, epoch: c.next, snapshots: make(map[string]sourceSnapshot)}
		c.records[key] = record
	}
	record.refs++
	return &sourceLease{owner: c, record: record}
}

func (l *sourceLease) release() {
	if l == nil {
		return
	}
	c := l.owner
	c.mu.Lock()
	defer c.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	l.record.refs--
	c.discardUnused(l.record)
}

func (c *sourceCoordinator) discardUnused(record *sourceRecord) {
	if record.refs == 0 && len(record.snapshots) == 0 {
		delete(c.records, record.key)
	}
}

func (l *sourceLease) version() uint64 {
	if l == nil {
		return 0
	}
	c := l.owner
	c.mu.Lock()
	defer c.mu.Unlock()
	if l.released {
		return 0
	}
	return l.record.epoch
}

// invalidate never creates a record, and advances the live installation before removing its snapshots.
func (c *sourceCoordinator) invalidate(id string, identity any) {
	if identity != nil && !reflect.TypeOf(identity).Comparable() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	record := c.records[scopeIdentity{extID: id, identity: identity}]
	if record == nil {
		return
	}
	c.next++
	record.epoch = c.next
	for key := range record.snapshots {
		c.remove(record, key)
	}
	c.discardUnused(record)
}

func (c *sourceCoordinator) remove(record *sourceRecord, key string) {
	snapshot, ok := record.snapshots[key]
	if !ok {
		return
	}
	c.entries--
	c.bytes -= snapshot.bytes
	delete(record.snapshots, key)
}

func (l *sourceLease) get(key string) (CacheEntry, uint64, bool) {
	if l == nil {
		return CacheEntry{}, 0, false
	}
	c := l.owner
	c.mu.Lock()
	defer c.mu.Unlock()
	if l.released {
		return CacheEntry{}, 0, false
	}
	snapshot, ok := l.record.snapshots[key]
	if !ok {
		return CacheEntry{}, l.record.epoch, false
	}
	data, err := json.Marshal(snapshot.entry.Items)
	if err != nil {
		return CacheEntry{}, l.record.epoch, false
	}
	var items []api.Item
	if json.Unmarshal(data, &items) != nil {
		return CacheEntry{}, l.record.epoch, false
	}
	return CacheEntry{Items: items, At: snapshot.entry.At}, l.record.epoch, true
}

// put admits a result only while its lease and captured epoch remain current.
func (l *sourceLease) put(key string, epoch uint64, entry CacheEntry) bool {
	if l == nil {
		return false
	}
	encoded, err := json.Marshal(entry.Items)
	if err != nil {
		return false
	}
	var items []api.Item
	if err := json.Unmarshal(encoded, &items); err != nil {
		return false
	}
	size := len(encoded) + len(key)
	c := l.owner
	c.mu.Lock()
	defer c.mu.Unlock()
	if l.released || l.record.epoch != epoch {
		return false
	}
	c.remove(l.record, key)
	if size > c.maxBytes || c.maxEntries <= 0 {
		return true
	}
	for c.entries >= c.maxEntries || c.bytes+size > c.maxBytes {
		if !c.evictOne() {
			break
		}
	}
	l.record.snapshots[key] = sourceSnapshot{entry: CacheEntry{Items: items, At: entry.At}, bytes: size}
	c.entries++
	c.bytes += size
	return true
}

func (c *sourceCoordinator) evictOne() bool {
	for _, record := range c.records {
		for key := range record.snapshots {
			c.remove(record, key)
			c.discardUnused(record)
			return true
		}
	}
	return false
}
