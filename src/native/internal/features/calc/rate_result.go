package calc

import (
	"math"
	"strings"
	"time"
)

// rateStaleAfter is how old a cached quote may be before it counts as stale. Stale quotes still answer; the result
// is marked so the view can say so, and a refresh is queued.
const rateStaleAfter = 24 * time.Hour

// calculateRateResult prices a parsed currency question with a quote. The quote's age decides whether the result is
// stale and what its subtitle says.
func calculateRateResult(query string, parsed RateExpression, quote Quote, now time.Time) (Result, bool) {
	value := normalizeFloatingPoint(parsed.Amount * quote.Rate)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Result{}, false
	}
	rawAmount := formatRawNumber(value, 4)
	perUnit := ""
	if parsed.RateUnit != "" {
		perUnit = "/" + parsed.RateUnit
	}
	raw := rawAmount + " " + parsed.TargetCurrency + perUnit
	display := formatCurrencyAmount(value, parsed.TargetCurrency) + perUnit
	subtitle := "Copy converted result to clipboard · " + quote.Provider
	var age time.Duration
	if !quote.FetchedAt.IsZero() {
		age = now.Sub(quote.FetchedAt)
		if label := ageLabel(age); label != "" {
			subtitle += " · " + label
		}
	}
	r := newResult(KindRate, strings.TrimSpace(query), parsed.Expression, value, raw, display, subtitle)
	r.Unit = parsed.SourceCurrency
	r.TargetUnit = parsed.TargetCurrency
	r.RateUnit = parsed.RateUnit
	r.Source = quote.Provider
	r.SwapQuery = rawAmount + " " + parsed.TargetCurrency + perUnit + " to " + parsed.SourceCurrency
	r.CacheAge = age
	r.Stale = quoteIsStale(quote, now)
	return describe(r), true
}

// quoteIsStale reports whether a quote is older than the refresh window, or undated.
func quoteIsStale(quote Quote, now time.Time) bool {
	return quote.FetchedAt.IsZero() || now.Sub(quote.FetchedAt) >= rateStaleAfter
}

// formatCurrencyAmount renders an amount in a currency the way the en-US currency style does: the symbol, grouped
// thousands and two decimals (none for yen), at most four decimals.
func formatCurrencyAmount(value float64, code string) string {
	body := formatDecimal(math.Abs(value), currencyFractionDigits(code), 4, true)
	text := currencyText(code, body)
	if value < 0 {
		return "-" + text
	}
	return text
}
