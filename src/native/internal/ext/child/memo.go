package child

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// memoPrefix namespaces the entries Memo writes, so they cannot be mistaken for keys the extension set itself.
const memoPrefix = "memo/"

// memoEnvelope is what Memo stores: the value, when it stops being fresh, and when it stops being usable as stale.
type memoEnvelope struct {
	Value      json.RawMessage `json:"value"`
	FreshUntil int64           `json:"freshUntilMs"`
	StaleUntil int64           `json:"staleUntilMs"`
}

// memoKV is the key-value store Memo keeps its envelopes in. The storage service implements it in production.
type memoKV interface {
	read(key string) (json.RawMessage, bool, error)
	write(key string, value any) error
}

// memoCore implements Storage.Memo and Storage.MemoStale over any memoKV.
type memoCore struct {
	kv      memoKV
	clock   platform.Clock
	flights *flightGroup
}

// Memo returns the stored value while it is fresh; otherwise it runs load and stores the result for ttl.
func (m memoCore) Memo(key string, ttl time.Duration, into any, load func() (any, error)) error {
	return m.MemoStale(key, ttl, 0, into, load)
}

// MemoStale returns fresh data, or stale data up to staleTTL while a single background refresh runs. Without
// usable stale data the load runs before the call returns.
func (m memoCore) MemoStale(key string, ttl, staleTTL time.Duration, into any, load func() (any, error)) error {
	envelope, found, err := m.readEnvelope(key)
	if err != nil {
		return err
	}
	now := m.clock.Now().UnixMilli()
	if found && now < envelope.FreshUntil {
		return decodeInto(envelope.Value, into)
	}
	if found && now < envelope.StaleUntil {
		m.refreshInBackground(key, ttl, staleTTL, load)
		return decodeInto(envelope.Value, into)
	}
	return m.loadAndStore(key, ttl, staleTTL, into, load)
}

func (m memoCore) loadAndStore(key string, ttl, staleTTL time.Duration, into any, load func() (any, error)) error {
	value, err := load()
	if err != nil {
		return err
	}
	if err := m.store(key, value, ttl, staleTTL); err != nil {
		return err
	}
	return decodeInto(mustMarshal(value), into)
}

// refreshInBackground starts one refresh per key. The refresh runs on its own goroutine: the load function must
// not depend on state the extension's main path is changing.
func (m memoCore) refreshInBackground(key string, ttl, staleTTL time.Duration, load func() (any, error)) {
	if !m.flights.begin(key) {
		return
	}
	safego.Go("ext.memo.refresh", func(_ context.Context) {
		defer m.flights.end(key)
		value, err := load()
		if err == nil {
			_ = m.store(key, value, ttl, staleTTL)
		}
	})
}

func (m memoCore) store(key string, value any, ttl, staleTTL time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	now := m.clock.Now()
	envelope := memoEnvelope{
		Value:      raw,
		FreshUntil: now.Add(ttl).UnixMilli(),
		StaleUntil: now.Add(ttl + staleTTL).UnixMilli(),
	}
	return m.kv.write(memoPrefix+key, envelope)
}

func (m memoCore) readEnvelope(key string) (memoEnvelope, bool, error) {
	raw, found, err := m.kv.read(memoPrefix + key)
	if err != nil || !found {
		return memoEnvelope{}, false, err
	}
	var envelope memoEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return memoEnvelope{}, false, nil
	}
	return envelope, true, nil
}

// decodeInto unmarshals a memoized value into the caller's pointer. A nil target accepts and drops the value.
func decodeInto(raw json.RawMessage, into any) error {
	if into == nil {
		return nil
	}
	return json.Unmarshal(raw, into)
}

func mustMarshal(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}

// flightGroup remembers which keys have a background refresh running, so a key refreshes once at a time.
type flightGroup struct {
	mu     sync.Mutex
	active map[string]bool
}

func newFlightGroup() *flightGroup {
	return &flightGroup{active: map[string]bool{}}
}

// begin reports whether the caller may start the refresh for key; false means one is already running.
func (g *flightGroup) begin(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active[key] {
		return false
	}
	g.active[key] = true
	return true
}

func (g *flightGroup) end(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.active, key)
}
