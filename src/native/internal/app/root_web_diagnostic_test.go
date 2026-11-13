package app

import (
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
)

type observedWebSearch struct {
	websearch.Extension
	mu      sync.Mutex
	records []webSearchCall
}

type webSearchCall struct {
	query   string
	started time.Time
	ended   time.Time
	items   int
	err     error
}

func newObservedWebSearch(t *testing.T, extension websearch.Extension) *observedWebSearch {
	t.Helper()
	observed := &observedWebSearch{Extension: extension}
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		observed.mu.Lock()
		records := append([]webSearchCall(nil), observed.records...)
		observed.mu.Unlock()
		for _, record := range records {
			state := "completed"
			elapsed := record.ended.Sub(record.started)
			if record.ended.IsZero() {
				state = "in-flight when diagnostics were collected"
				elapsed = time.Since(record.started)
			}
			t.Logf("web search provider query %q %s: elapsed=%s items=%d error=%v", record.query, state, elapsed, record.items, record.err)
		}
	})
	return observed
}

func (e *observedWebSearch) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	call := webSearchCall{query: query, started: time.Now()}
	e.mu.Lock()
	e.records = append(e.records, call)
	index := len(e.records) - 1
	e.mu.Unlock()
	items, err := e.Extension.SearchItems(ctx, query)
	ended := time.Now()
	e.mu.Lock()
	e.records[index].ended = ended
	e.records[index].items = len(items)
	e.records[index].err = err
	e.mu.Unlock()
	return items, err
}
