package calculator

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/search"
)

var testNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// fixedClock reads the same instant every time.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// memCache is an in-memory rate cache that records its saves.
type memCache struct {
	mu    sync.Mutex
	rates calc.Rates
	saved chan calc.Rates
}

func (m *memCache) Load() (calc.Rates, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := calc.Rates{}
	for key, quote := range m.rates {
		copied[key] = quote
	}
	return copied, nil
}

func (m *memCache) Save(rates calc.Rates) error {
	m.mu.Lock()
	m.rates = rates
	m.mu.Unlock()
	if m.saved != nil {
		m.saved <- rates
	}
	return nil
}

// fakeNetwork answers every fetch with one body.
type fakeNetwork struct {
	status int
	body   string
	urls   chan string
}

func (n fakeNetwork) Fetch(request api.FetchRequest) (api.FetchResponse, error) {
	if n.urls != nil {
		n.urls <- request.URL
	}
	return api.FetchResponse{Status: n.status, Body: n.body}, nil
}

// fakeViews records the queries the calculator asked to search again.
type fakeViews struct {
	api.Views
	refreshed chan string
}

func (v fakeViews) RefreshSearch(query string) { v.refreshed <- query }

// fakeContext serves the clock, the network and the views; every other service is absent.
type fakeContext struct {
	api.Context
	clock   api.Clock
	network api.Network
	views   api.Views
}

func (c fakeContext) Std() context.Context { return context.Background() }
func (c fakeContext) Clock() api.Clock     { return c.clock }
func (c fakeContext) Network() api.Network { return c.network }
func (c fakeContext) Views() api.Views     { return c.views }

func contextAt(now time.Time, network api.Network, views api.Views) fakeContext {
	return fakeContext{clock: fixedClock{now: now}, network: network, views: views}
}

// staleQuote is a USD/EUR quote fetched three days before testNow.
func staleQuote() calc.Rates {
	fetched := testNow.Add(-72 * time.Hour)
	return calc.Rates{"USD/EUR": {Rate: 0.5, Provider: "Frankfurter", UpdatedAt: fetched, FetchedAt: fetched}}
}

func TestStaleRateSubtitleNamesTheCacheAge(t *testing.T) {
	ext := New(&memCache{rates: staleQuote()}, time.UTC)
	items, err := ext.SearchItems(contextAt(testNow, nil, nil), "10 usd to eur")
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %v, err %v; want one answer", items, err)
	}
	if !strings.Contains(items[0].Subtitle, "3d old") {
		t.Fatalf("subtitle = %q, want the cache age", items[0].Subtitle)
	}
	if items[0].Section != search.KindCalculator || !items[0].AnswersQuery || items[0].Score != score {
		t.Fatalf("row = %+v, want a calculator answer ranked %d", items[0], score)
	}
}

func TestRateRefreshSavesTheQuoteAndSearchesAgain(t *testing.T) {
	cache := &memCache{saved: make(chan calc.Rates, 1)}
	refreshed := make(chan string, 1)
	urls := make(chan string, 1)
	network := fakeNetwork{status: 200, body: `{"rate": 0.5, "date": "2026-01-01"}`, urls: urls}
	ext := New(cache, time.UTC)
	ctx := contextAt(testNow, network, fakeViews{refreshed: refreshed})

	if items, _ := ext.SearchItems(ctx, "10 usd to eur"); len(items) != 0 {
		t.Fatalf("an answer appeared before any quote was fetched: %v", items)
	}
	if got := <-urls; got != calc.RateAddress("USD", "EUR") {
		t.Fatalf("fetched %q, want the USD/EUR quote", got)
	}
	saved := <-cache.saved
	if got := saved["USD/EUR"].Rate; got != 0.5 {
		t.Fatalf("saved rate = %v, want 0.5", got)
	}
	if query := <-refreshed; query != "10 usd to eur" {
		t.Fatalf("searched again for %q", query)
	}
}

func TestCopyAndSetQueryActionsFollowTheNativeOrder(t *testing.T) {
	items, _ := New(nil, time.UTC).SearchItems(contextAt(testNow, nil, nil), "15% of 240 + 3*(2^5)")
	if len(items) != 1 {
		t.Fatalf("items = %v, want one answer", items)
	}
	var titles []string
	for _, action := range items[0].Actions {
		titles = append(titles, action.Title)
	}
	want := "Continue Calculation|Paste Result"
	if got := strings.Join(titles, "|"); got != want {
		t.Fatalf("actions = %q, want %q", got, want)
	}
	if items[0].Primary == nil || items[0].Primary.Title != "Copy Result" || items[0].Primary.Text != "132" {
		t.Fatalf("primary = %+v, want Copy Result of 132", items[0].Primary)
	}
}

func TestDetailFactsAreTheDetailLinesInOrder(t *testing.T) {
	items, _ := New(nil, time.UTC).SearchItems(contextAt(testNow, nil, nil), "15% of 240 + 3*(2^5)")
	facts := items[0].Detail.Facts
	if len(facts) < 2 || facts[0].Label != "Expression" || facts[1].Label != "Result" || facts[1].Value != "132" {
		t.Fatalf("facts = %+v, want Expression then Result 132", facts)
	}
}

func TestRateRefreshIsSkippedWithoutAQueuedPair(t *testing.T) {
	ext := New(nil, time.UTC)
	items, _ := ext.SearchItems(contextAt(testNow, nil, nil), "2 + 2")
	if len(items) != 1 || items[0].Title != "2 + 2 = 4" {
		t.Fatalf("items = %+v, want the arithmetic answer", items)
	}
}

func TestManifestDeclaresTheNetworkCapability(t *testing.T) {
	manifest := New(nil, time.UTC).Manifest()
	if len(manifest.Capabilities) != 1 || manifest.Capabilities[0] != api.CapabilityNetwork {
		t.Fatalf("capabilities = %v, want network", manifest.Capabilities)
	}
	if _, err := json.Marshal(manifest); err != nil {
		t.Fatal(err)
	}
}
