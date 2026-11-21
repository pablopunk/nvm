package calc

import (
	"testing"
	"time"
)

// TestParsesAndFormatsCachedFiatRates ports the cached fiat rate test: parsing of the currency forms, and pricing
// with a quote.
func TestParsesAndFormatsCachedFiatRates(t *testing.T) {
	quote := Quote{Rate: 0.8, Provider: "Frankfurter", UpdatedAt: testNow, FetchedAt: testNow}
	type parsedCase struct {
		query          string
		amount         float64
		sourceCurrency string
		targetCurrency string
		rateUnit       string
	}
	parsed := []parsedCase{
		{"10 usd in gbp", 10, "USD", "GBP", ""},
		{"$1.2m in gbp", 1_200_000, "USD", "GBP", ""},
		{"8 dollars/hour in gbp", 8, "USD", "GBP", "hour"},
		{"10 $", 10, "USD", "EUR", ""},
		{"$10", 10, "USD", "EUR", ""},
		{"10 €", 10, "EUR", "USD", ""},
		{"€10", 10, "EUR", "USD", ""},
	}
	for _, tc := range parsed {
		t.Run("parse "+tc.query, func(t *testing.T) {
			got, ok := parseRateExpression(tc.query)
			if !ok {
				t.Fatalf("parseRateExpression(%q) returned no expression", tc.query)
			}
			if got.Amount != tc.amount || got.SourceCurrency != tc.sourceCurrency || got.TargetCurrency != tc.targetCurrency || got.RateUnit != tc.rateUnit {
				t.Fatalf("parseRateExpression(%q) = %+v", tc.query, got)
			}
		})
	}

	priced := []struct {
		query string
		raw   string
	}{
		{"10 usd in gbp", "8 GBP"},
		{"$1.2m in gbp", "960000 GBP"},
		{"8 dollars/hour in gbp", "6.4 GBP/hour"},
	}
	for _, tc := range priced {
		t.Run("price "+tc.query, func(t *testing.T) {
			parsedRate, ok := parseRateExpression(tc.query)
			if !ok {
				t.Fatalf("parseRateExpression(%q) returned no expression", tc.query)
			}
			got, ok := calculateRateResult(tc.query, parsedRate, quote, testNow)
			if !ok || got.Raw != tc.raw {
				t.Fatalf("calculateRateResult(%q).Raw = %q (ok %v), want %q", tc.query, got.Raw, ok, tc.raw)
			}
		})
	}
}

// TestRateResultLabelsSubtitleAndSwap checks the presentation fields the palette shows beside a converted amount.
func TestRateResultLabelsSubtitleAndSwap(t *testing.T) {
	parsedRate, _ := parseRateExpression("8 dollars/hour in gbp")
	got, ok := calculateRateResult("8 dollars/hour in gbp", parsedRate, fiatQuote(0.8, testNow.Add(-3*time.Hour)), testNow)
	if !ok {
		t.Fatal("calculateRateResult returned no result")
	}
	if got.Display != "£6.40/hour" {
		t.Fatalf("Display = %q", got.Display)
	}
	if got.Title != "8 dollars/hour in gbp = £6.40/hour" {
		t.Fatalf("Title = %q", got.Title)
	}
	if got.Subtitle != "Copy converted result to clipboard · Frankfurter · 3h old" {
		t.Fatalf("Subtitle = %q", got.Subtitle)
	}
	if got.SwapQuery != "6.4 GBP/hour to USD" {
		t.Fatalf("SwapQuery = %q", got.SwapQuery)
	}
	if got.Stale || got.CacheAge != 3*time.Hour {
		t.Fatalf("Stale %v CacheAge %v, want fresh 3h", got.Stale, got.CacheAge)
	}
}

// TestParseRateExpressionRejectsNonCurrencyQuestions checks each shape the parser must refuse.
func TestParseRateExpressionRejectsNonCurrencyQuestions(t *testing.T) {
	for _, query := range []string{
		"10 usd in usd",
		"10 gbp",
		"usd in gbp",
		"10 usd in",
		"10 usd in gbp/",
		"hello in gbp",
		"10 ft in m",
		"",
	} {
		if got, ok := parseRateExpression(query); ok {
			t.Errorf("parseRateExpression(%q) = %+v, want no expression", query, got)
		}
	}
}

// TestRateResponseParsing checks which rate responses are accepted and which are errors.
func TestRateResponseParsing(t *testing.T) {
	now := testNow
	accepted := []struct {
		name string
		body string
		rate float64
		when time.Time
	}{
		{"numeric string rate", `{"rate":"0.8","date":"2026-06-06"}`, 0.8, time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)},
		{"updated_at when date is absent", `{"rate":2,"updated_at":"2026-06-05T10:00:00Z"}`, 2, time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)},
		{"unparseable date falls back to now", `{"rate":2,"date":"soon"}`, 2, now},
		{"no date falls back to now", `{"rate":2}`, 2, now},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			rate, when, err := parseRateBody([]byte(tc.body), now)
			if err != nil || rate != tc.rate || !when.Equal(tc.when) {
				t.Fatalf("parseRateBody = %v, %v, %v; want %v at %v", rate, when, err, tc.rate, tc.when)
			}
		})
	}
	rejected := map[string]string{
		"zero rate":        `{"rate":0}`,
		"negative rate":    `{"rate":-1}`,
		"missing rate":     `{"date":"2026-06-06"}`,
		"text rate":        `{"rate":"abc"}`,
		"boolean rate":     `{"rate":true}`,
		"not json":         `<html>`,
		"array body":       `[1,2]`,
		"infinite numeric": `{"rate":1e400}`,
	}
	for name, body := range rejected {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseRateBody([]byte(body), now); err == nil {
				t.Fatalf("parseRateBody(%s) accepted the body", body)
			}
		})
	}
}
