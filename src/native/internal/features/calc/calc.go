package calc

import (
	"context"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Calculator answers calculator queries: arithmetic, percentages, unit conversions, calendar and clock questions,
// and currency conversions from cached rates. It reads the time only through its Clock, and it never blocks on the
// network. It is safe for concurrent use.
type Calculator struct {
	clock platform.Clock
	zone  *time.Location
	rates *RateService
	zones zoneCache
}

// New returns a calculator. Calendar answers are computed in zone, or the machine's local zone when zone is nil.
// rates may be nil, which turns currency answers off.
func New(clock platform.Clock, zone *time.Location, rates *RateService) *Calculator {
	if zone == nil {
		zone = time.Local
	}
	return &Calculator{clock: clock, zone: zone, rates: rates}
}

// Evaluate answers a query that is arithmetic, a percentage, a unit conversion, or a calendar or clock question. It
// never reads rates, so it is the offline core: a query it does not answer returns false.
func (c *Calculator) Evaluate(input string) (Result, bool) {
	query := strings.Trim(canonicalSpace(input), " ")
	normalized, ok := normalizeCalculationQuery(query)
	if !ok || !isLikelyCalculation(normalized.expression, normalized.explicit) {
		return Result{}, false
	}
	expression := normalized.expression
	if hasASCIILetter(expression) {
		if r, ok := c.calculateWords(expression, query); ok {
			return r, true
		}
	}
	return calculateMath(query, expression)
}

// calculateWords tries the answers that need letters: calendar questions, clock questions, conversions and
// percentage changes with units.
func (c *Calculator) calculateWords(expression, query string) (Result, bool) {
	now := c.clock.Now()
	if r, ok := calculateDate(expression, query, dateCalendar{now: now, zone: c.zone}); ok {
		return r, true
	}
	if r, ok := calculateTimezone(&c.zones, expression, query, now); ok {
		return r, true
	}
	if r, ok := calculateConversion(expression, query); ok {
		return r, true
	}
	return calculateUnitPercent(expression, query)
}

// calculateMath evaluates arithmetic. The title keeps the expression as the user typed it; the result's expression
// is the rewritten form the evaluator read.
func calculateMath(query, expression string) (Result, bool) {
	natural := normalizeNaturalMath(expression)
	value, ok := evaluateMathExpression(natural)
	if !ok {
		return Result{}, false
	}
	kind := KindMath
	if strings.Contains(natural, "%") {
		kind = KindPercent
	}
	r := newResult(kind, strings.TrimSpace(query), expression, value, formatRawNumber(value, normalizedPrecision), formatDisplayNumber(value, normalizedPrecision), "Copy result to clipboard")
	r.Expression = natural
	return describe(r), true
}

// Query answers a query in full: everything Evaluate answers, then currency conversions priced from the cached
// rates. A currency pair with no quote yet is queued for the next Refresh, and the query returns no answer until a
// quote arrives. Query never waits on the network.
func (c *Calculator) Query(input string) (Result, bool) {
	if r, ok := c.Evaluate(input); ok {
		return r, true
	}
	if c.rates == nil {
		return Result{}, false
	}
	query := strings.Trim(canonicalSpace(input), " ")
	parsed, ok := parseRateExpression(query)
	if !ok {
		return Result{}, false
	}
	c.rates.queue(parsed.SourceCurrency, parsed.TargetCurrency)
	quote, ok := c.rates.Quote(parsed.SourceCurrency, parsed.TargetCurrency)
	if !ok {
		return Result{}, false
	}
	return calculateRateResult(query, parsed, quote, c.clock.Now())
}

// Refresh fetches the rate pairs that Query queued since the last call and stores them in the cache. It is the
// background hook: call it from a goroutine after a currency query, and it returns the first error it met while
// keeping every quote it did get. It is a no-op when the calculator has no rate service.
func (c *Calculator) Refresh(ctx context.Context) error {
	if c.rates == nil {
		return nil
	}
	return c.rates.Refresh(ctx)
}
