package calc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Quote is one exchange rate: how many units of the target currency one unit of the source currency buys.
type Quote struct {
	Rate      float64
	Provider  string
	UpdatedAt time.Time
	FetchedAt time.Time
}

// Rates is the cache content: quotes keyed by "BASE/QUOTE" in upper case.
type Rates map[string]Quote

// RateCache persists quotes between runs. Load returns what was saved last; Save replaces it. The calculator never
// reads the store package directly; the composition root supplies an implementation.
type RateCache interface {
	Load() (Rates, error)
	Save(Rates) error
}

// currencyPair is one source and target currency queued for refresh.
type currencyPair struct {
	source string
	target string
}

// RateService holds the cached quotes, queues the pairs users ask about, and refreshes them over HTTP. Quotes are
// served from memory without waiting on the network. A refresh that fails keeps the quote it had.
type RateService struct {
	cache RateCache

	refreshing sync.Mutex
	mu         sync.Mutex
	rates      Rates
	queued     []currencyPair
	queuedKeys map[string]bool
}

// NewRateService returns a service seeded from cache. A nil cache keeps quotes in memory only. When loading fails
// the service still works, starts empty, and the load error is returned so the caller can report it.
func NewRateService(cache RateCache) (*RateService, error) {
	s := &RateService{
		cache:      cache,
		rates:      Rates{},
		queuedKeys: map[string]bool{},
	}
	if cache == nil {
		return s, nil
	}
	loaded, err := cache.Load()
	if err != nil {
		return s, fmt.Errorf("load rate cache: %w", err)
	}
	for key, quote := range loaded {
		if validRate(quote.Rate) {
			s.rates[key] = quote
		}
	}
	return s, nil
}

// Quote returns the cached quote for source to target. A quote stored the other way round is inverted.
func (s *RateService) Quote(source, target string) (Quote, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return quoteFrom(s.rates, source, target)
}

// quoteFrom looks a pair up directly, then in reverse.
func quoteFrom(rates Rates, source, target string) (Quote, bool) {
	if quote, ok := rates[rateKey(source, target)]; ok && validRate(quote.Rate) {
		return quote, true
	}
	if quote, ok := rates[rateKey(target, source)]; ok && validRate(quote.Rate) {
		quote.Rate = 1 / quote.Rate
		return quote, true
	}
	return Quote{}, false
}

// queue records a pair that a user asked about, once, until the next Refresh. It does no I/O.
func (s *RateService) queue(source, target string) {
	key := rateKey(source, target)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queuedKeys[key] {
		return
	}
	s.queuedKeys[key] = true
	s.queued = append(s.queued, currencyPair{source: source, target: target})
}

// Pending reports whether pairs are queued for the next Refresh.
func (s *RateService) Pending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queued) > 0
}

// Refresh fetches every queued pair whose quote is missing or older than a day, through fetch, stores the results,
// and saves the cache when anything changed. Pairs are dequeued whether or not they succeed, so a failing currency is
// not retried until someone asks for it again. Errors are joined; quotes that succeeded are kept. A call made while
// another refresh runs returns at once.
func (s *RateService) Refresh(ctx context.Context, clock Clock, fetch RateFetcher) error {
	if !s.refreshing.TryLock() {
		return nil
	}
	defer s.refreshing.Unlock()
	var errs []error
	changed := false
	for _, pair := range s.takeQueued() {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if s.isFresh(pair, clock.Now()) {
			continue
		}
		quote, err := s.fetchQuote(ctx, fetch, clock, pair)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", pair.source, pair.target, err))
			continue
		}
		s.store(pair, quote)
		changed = true
	}
	if changed && s.cache != nil {
		if err := s.cache.Save(s.snapshot()); err != nil {
			errs = append(errs, fmt.Errorf("save rate cache: %w", err))
		}
	}
	return errors.Join(errs...)
}

// takeQueued empties the queue and returns what it held.
func (s *RateService) takeQueued() []currencyPair {
	s.mu.Lock()
	defer s.mu.Unlock()
	pairs := s.queued
	s.queued = nil
	s.queuedKeys = map[string]bool{}
	return pairs
}

// isFresh reports whether the direct quote for a pair was fetched within the stale window.
func (s *RateService) isFresh(pair currencyPair, now time.Time) bool {
	s.mu.Lock()
	quote, ok := s.rates[rateKey(pair.source, pair.target)]
	s.mu.Unlock()
	return ok && !quote.FetchedAt.IsZero() && now.Sub(quote.FetchedAt) < rateStaleAfter
}

// store records a fresh quote for a pair.
func (s *RateService) store(pair currencyPair, quote Quote) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rates[rateKey(pair.source, pair.target)] = quote
}

// snapshot copies the current quotes for saving.
func (s *RateService) snapshot() Rates {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make(Rates, len(s.rates))
	for key, quote := range s.rates {
		copied[key] = quote
	}
	return copied
}

// rateKey is the cache key for a base and quote currency.
func rateKey(base, quote string) string {
	return strings.ToUpper(base) + "/" + strings.ToUpper(quote)
}

// validRate reports whether a rate is a finite positive number, the only kind a quote may hold.
func validRate(rate float64) bool {
	return rate > 0 && !math.IsInf(rate, 0) && !math.IsNaN(rate)
}
