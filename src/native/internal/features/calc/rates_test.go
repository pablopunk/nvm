package calc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// memCache is an in-memory RateCache that counts saves and can be told to fail.
type memCache struct {
	rates    Rates
	loadErr  error
	saveErr  error
	saves    int
	loadedAt int
}

func (m *memCache) Load() (Rates, error) {
	m.loadedAt++
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	copied := Rates{}
	for key, quote := range m.rates {
		copied[key] = quote
	}
	return copied, nil
}

func (m *memCache) Save(rates Rates) error {
	m.saves++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.rates = rates
	return nil
}

// fiatQuote builds a cache entry for USD/GBP fetched at fetchedAt.
func fiatQuote(rate float64, fetchedAt time.Time) Quote {
	return Quote{Rate: rate, Provider: "Frankfurter", UpdatedAt: fetchedAt, FetchedAt: fetchedAt}
}

// rateBodyJSON is a successful response from the rate source.
const rateBodyJSON = `{"rate":0.8,"date":"2026-06-06"}`

// blockingHTTP never answers; it returns when the request context is done, as a hung connection would.
type blockingHTTP struct{}

func (blockingHTTP) Do(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

// TestRateServiceServesFreshCacheWithoutNetwork checks a fresh cached quote answers at once and a refresh does not
// touch the network for it.
func TestRateServiceServesFreshCacheWithoutNetwork(t *testing.T) {
	clock := clockAt(testNow)
	h := fake.NewHTTP()
	cache := &memCache{rates: Rates{"USD/GBP": fiatQuote(0.8, testNow.Add(-time.Hour))}}
	rates := mustRateService(t, h, clock, cache)
	c := New(clock, time.UTC, rates)

	got, ok := c.Query("10 usd in gbp")
	if !ok || got.Raw != "8 GBP" || got.Stale {
		t.Fatalf("Query = %+v, ok %v; want fresh 8 GBP", got, ok)
	}
	if err := c.Refresh(context.Background(), clock, fetchFrom(h)); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := len(h.Requests()); n != 0 {
		t.Fatalf("fresh quote caused %d requests", n)
	}
}

// TestRateServiceServesStaleQuoteOfflineAndLabelsIt checks an old cached quote still answers when the network is
// down, is marked stale with its age, and survives the failed refresh unchanged.
func TestRateServiceServesStaleQuoteOfflineAndLabelsIt(t *testing.T) {
	clock := clockAt(testNow)
	h := fake.NewHTTP()
	h.Script(fake.Response{Err: errors.New("network unreachable")})
	cache := &memCache{rates: Rates{"USD/GBP": fiatQuote(0.8, testNow.Add(-72*time.Hour))}}
	rates := mustRateService(t, h, clock, cache)
	c := New(clock, time.UTC, rates)

	got, ok := c.Query("10 usd in gbp")
	if !ok || got.Raw != "8 GBP" {
		t.Fatalf("Query = %+v, ok %v; want the stale 8 GBP", got, ok)
	}
	if !got.Stale || got.CacheAge != 72*time.Hour {
		t.Fatalf("Stale %v CacheAge %v, want stale 72h", got.Stale, got.CacheAge)
	}
	if !strings.HasSuffix(got.Subtitle, "· 3d old") {
		t.Fatalf("Subtitle = %q, want the 3d old label", got.Subtitle)
	}
	if err := c.Refresh(context.Background(), clock, fetchFrom(h)); err == nil {
		t.Fatal("Refresh returned nil while offline")
	}
	if cache.saves != 0 {
		t.Fatalf("failed refresh saved the cache %d times", cache.saves)
	}
	again, ok := c.Query("10 usd in gbp")
	if !ok || again.Raw != "8 GBP" {
		t.Fatalf("after failed refresh Query = %+v, ok %v", again, ok)
	}
}

// TestRateServiceStaleWindowIsExactlyOneDay checks the boundary: a quote is stale from 24 hours on, not before.
func TestRateServiceStaleWindowIsExactlyOneDay(t *testing.T) {
	if quoteIsStale(fiatQuote(1, testNow.Add(-rateStaleAfter+time.Second)), testNow) {
		t.Fatal("quote just under a day old is stale")
	}
	if !quoteIsStale(fiatQuote(1, testNow.Add(-rateStaleAfter)), testNow) {
		t.Fatal("quote exactly a day old is not stale")
	}
	if !quoteIsStale(Quote{Rate: 1}, testNow) {
		t.Fatal("undated quote is not stale")
	}
}

// TestRateServiceRefreshStoresQuoteAndSavesCache checks a missing quote is fetched once from the source, stored,
// saved, and then answers.
func TestRateServiceRefreshStoresQuoteAndSavesCache(t *testing.T) {
	clock := clockAt(testNow)
	h := fake.NewHTTP()
	h.Script(fake.Response{Body: []byte(rateBodyJSON)})
	cache := &memCache{}
	rates := mustRateService(t, h, clock, cache)
	c := New(clock, time.UTC, rates)

	if _, ok := c.Query("10 usd in gbp"); ok {
		t.Fatal("Query answered without a quote")
	}
	if err := c.Refresh(context.Background(), clock, fetchFrom(h)); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requests := h.Requests()
	if len(requests) != 1 || requests[0].URL != "https://api.frankfurter.dev/v2/rate/USD/GBP" {
		t.Fatalf("requests = %+v", requests)
	}
	if cache.saves != 1 || cache.rates["USD/GBP"].Rate != 0.8 {
		t.Fatalf("saved %d times with %+v", cache.saves, cache.rates)
	}
	got, ok := c.Query("10 usd in gbp")
	if !ok || got.Raw != "8 GBP" || got.Stale {
		t.Fatalf("Query after refresh = %+v, ok %v", got, ok)
	}
}

// TestRateServiceKeepsOldQuoteWhenSourceFails checks a server error leaves the stored quote and the cache alone.
func TestRateServiceKeepsOldQuoteWhenSourceFails(t *testing.T) {
	clock := clockAt(testNow)
	h := fake.NewHTTP()
	h.Script(fake.Response{Status: http.StatusInternalServerError})
	old := fiatQuote(0.8, testNow.Add(-48*time.Hour))
	cache := &memCache{rates: Rates{"USD/GBP": old}}
	rates := mustRateService(t, h, clock, cache)
	c := New(clock, time.UTC, rates)

	c.Query("10 usd in gbp")
	err := c.Refresh(context.Background(), clock, fetchFrom(h))
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("Refresh error = %v, want HTTP 500", err)
	}
	quote, ok := rates.Quote("USD", "GBP")
	if !ok || quote.Rate != old.Rate || !quote.FetchedAt.Equal(old.FetchedAt) {
		t.Fatalf("quote after failed refresh = %+v, ok %v", quote, ok)
	}
	if cache.saves != 0 {
		t.Fatalf("failed refresh saved the cache")
	}
}

// TestRateServiceUnknownCurrencyIsNotRetriedUntilAskedAgain checks an unknown three-letter code yields no answer,
// is fetched once, and is not refetched by a later refresh unless a query asks for it again.
func TestRateServiceUnknownCurrencyIsNotRetriedUntilAskedAgain(t *testing.T) {
	clock := clockAt(testNow)
	h := fake.NewHTTP()
	h.Script(fake.Response{Status: http.StatusNotFound})
	rates := mustRateService(t, h, clock, &memCache{})
	c := New(clock, time.UTC, rates)

	if _, ok := c.Query("10 abc in usd"); ok {
		t.Fatal("Query answered an unknown currency")
	}
	if err := c.Refresh(context.Background(), clock, fetchFrom(h)); err == nil {
		t.Fatal("Refresh of an unknown currency returned nil")
	}
	if err := c.Refresh(context.Background(), clock, fetchFrom(h)); err != nil {
		t.Fatalf("second Refresh without a query: %v", err)
	}
	if n := len(h.Requests()); n != 1 {
		t.Fatalf("unknown currency fetched %d times, want 1", n)
	}
}

// TestRateServiceInvertsReverseQuote checks a quote stored the other way round answers the pair in both directions.
func TestRateServiceInvertsReverseQuote(t *testing.T) {
	rates := mustRateService(t, fake.NewHTTP(), clockAt(testNow), &memCache{rates: Rates{"GBP/USD": fiatQuote(1.25, testNow)}})
	quote, ok := rates.Quote("USD", "GBP")
	if !ok || quote.Rate != 0.8 {
		t.Fatalf("inverse quote = %+v, ok %v; want rate 0.8", quote, ok)
	}
}

// TestRateServiceLoadFailureStartsEmptyAndReportsIt checks a cache that cannot be read leaves a working, empty
// service and reports the failure.
func TestRateServiceLoadFailureStartsEmptyAndReportsIt(t *testing.T) {
	clock := clockAt(testNow)
	cache := &memCache{loadErr: errors.New("corrupt file")}
	rates, err := NewRateService(cache)
	if err == nil || !strings.Contains(err.Error(), "corrupt file") {
		t.Fatalf("NewRateService error = %v", err)
	}
	if _, ok := rates.Quote("USD", "GBP"); ok {
		t.Fatal("empty service returned a quote")
	}
	if _, ok := New(clock, time.UTC, rates).Query("10 usd in gbp"); ok {
		t.Fatal("Query answered with an empty cache")
	}
}

// TestRateServiceRefreshIsSkippedWhenAlreadyRunning checks a second refresh started during the first returns at once.
func TestRateServiceRefreshIsSkippedWhenAlreadyRunning(t *testing.T) {
	clock := clockAt(testNow)
	rates := mustRateService(t, fake.NewHTTP(), clock, nil)
	c := New(clock, time.UTC, rates)
	c.Query("10 usd in gbp")
	release := make(chan struct{})
	hung := func(ctx context.Context, _ string) (int, []byte, error) {
		<-release
		return 0, nil, context.Canceled
	}

	done := make(chan error, 1)
	go func() { done <- c.Refresh(context.Background(), clock, hung) }()
	for rates.refreshing.TryLock() {
		rates.refreshing.Unlock()
		time.Sleep(time.Millisecond)
	}
	if err := c.Refresh(context.Background(), clock, hung); err != nil {
		t.Fatalf("overlapping Refresh = %v, want nil", err)
	}
	close(release)
	<-done
}

// TestCalculatorWithoutRatesAnswersOnlyOffline checks a calculator built without a rate service answers arithmetic
// and gives no answer to currency questions, without panicking.
func TestCalculatorWithoutRatesAnswersOnlyOffline(t *testing.T) {
	c := New(clockAt(testNow), nil, nil)
	if got, ok := c.Query("2 + 2"); !ok || got.Raw != "4" {
		t.Fatalf("Query(2 + 2) = %+v, ok %v", got, ok)
	}
	if _, ok := c.Query("10 usd in gbp"); ok {
		t.Fatal("currency answered without a rate service")
	}
	if err := c.Refresh(context.Background(), clockAt(testNow), fetchFrom(blockingHTTP{})); err != nil {
		t.Fatalf("Refresh without rates: %v", err)
	}
}

// fetchFrom serves a rate request from the fake HTTP client h, through the same exchange the host runs.
func fetchFrom(h interface {
	Do(*http.Request) (*http.Response, error)
}) RateFetcher {
	return func(ctx context.Context, address string) (int, []byte, error) {
		return platform.Fetch(ctx, h, address, RateBodyLimit)
	}
}

// mustRateService builds a rate service and fails the test on a load error.
func mustRateService(t *testing.T, h interface {
	Do(*http.Request) (*http.Response, error)
}, clock *fake.Clock, cache RateCache) *RateService {
	t.Helper()
	rates, err := NewRateService(cache)
	if err != nil {
		t.Fatalf("NewRateService: %v", err)
	}
	return rates
}
