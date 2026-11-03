package calc

import (
	"math"
	"strings"
	"testing"
	"time"
)

// TestNormalizeRoundsTiesAwayFromZero checks 12-digit normalization rounds exact ties up in magnitude, as
// toPrecision does, where strconv's half-to-even would round down.
func TestNormalizeRoundsTiesAwayFromZero(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{1234567890125, 1234567890130},
		{-1234567890125, -1234567890130},
		{0.1 + 0.2, 0.3},
		{3.0480000000000005, 3.048},
		{0, 0},
		{math.Copysign(0, -1), 0},
		{1e21, 1e21},
	}
	for _, tc := range cases {
		if got := normalizeFloatingPoint(tc.in); got != tc.want {
			t.Errorf("normalizeFloatingPoint(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if got := normalizeFloatingPoint(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("normalizeFloatingPoint(+Inf) = %v", got)
	}
}

// TestDisplayRoundingUsesShortestDecimalHalfExpand checks display rounding follows Intl.NumberFormat: the shortest
// decimal form is rounded with ties away from zero, so 1.005 shows as 1.01.
func TestDisplayRoundingUsesShortestDecimalHalfExpand(t *testing.T) {
	cases := []struct {
		value    float64
		frac     int
		want     string
		raw      string
		rawFrac  int
		rawLabel string
	}{
		{1.005, 2, "1.01", "1.01", 2, "1.005 rounds up"},
		{0.125, 2, "0.13", "0.13", 2, "0.125 rounds up"},
		{0.6, 0, "1", "1", 0, "0.6 to zero places"},
		{0.04, 1, "0", "0", 1, "0.04 to zero is zero"},
		{-0.04, 1, "-0", "-0", 1, "negative zero keeps its sign"},
		{70000, 4, "70,000", "70000", 4, "whole numbers group"},
		{1234.5678, 2, "1,234.57", "1234.5678", 4, "grouping and rounding"},
		{999.9999, 2, "1,000", "999.9999", 4, "carry into a new digit"},
		{1e21, 0, "1,000,000,000,000,000,000,000", "1e+21", 0, "huge whole numbers"},
	}
	for _, tc := range cases {
		if got := formatDisplayNumber(tc.value, tc.frac); got != tc.want {
			t.Errorf("formatDisplayNumber(%v, %d) = %q, want %q (%s)", tc.value, tc.frac, got, tc.want, tc.rawLabel)
		}
		if got := formatRawNumber(tc.value, tc.rawFrac); got != tc.raw {
			t.Errorf("formatRawNumber(%v, %d) = %q, want %q (%s)", tc.value, tc.rawFrac, got, tc.raw, tc.rawLabel)
		}
	}
}

// TestCurrencyAmountsUseSymbolsAndCurrencyDigits checks the currency display: symbol placement, two decimals except
// yen, sign before the symbol, and a code for currencies without a symbol.
func TestCurrencyAmountsUseSymbolsAndCurrencyDigits(t *testing.T) {
	cases := []struct {
		value float64
		code  string
		want  string
	}{
		{8, "GBP", "£8.00"},
		{6.4, "GBP", "£6.40"},
		{960000, "GBP", "£960,000.00"},
		{-8, "USD", "-$8.00"},
		{1234.5678, "JPY", "¥1,234.5678"},
		{12, "JPY", "¥12"},
		{5, "CHF", "CHF 5.00"},
		{1, "XYZ", "XYZ 1.00"},
	}
	for _, tc := range cases {
		if got := formatCurrencyAmount(tc.value, tc.code); got != tc.want {
			t.Errorf("formatCurrencyAmount(%v, %s) = %q, want %q", tc.value, tc.code, got, tc.want)
		}
	}
}

// TestNumberLiteralsAreStrict checks only plain decimals with optional sign and thousands commas parse.
func TestNumberLiteralsAreStrict(t *testing.T) {
	accepted := map[string]float64{"1,000": 1000, "+5": 5, "-5": -5, ".5": 0.5, "5.": 5, "1,234,567.25": 1234567.25}
	for text, want := range accepted {
		if got, ok := parseNumberLiteral(text); !ok || got != want {
			t.Errorf("parseNumberLiteral(%q) = %v, %v; want %v", text, got, ok, want)
		}
	}
	overflowing := "1" + strings.Repeat("0", 400)
	for _, text := range []string{"", ".", "+", "-", "1e5", "0x10", "NaN", "Inf", "1.2.3", "1 2", overflowing} {
		if got, ok := parseNumberLiteral(text); ok {
			t.Errorf("parseNumberLiteral(%q) = %v, want rejection", text, got)
		}
	}
}

// TestAmountSuffixes checks the k, m and b suffixes scale amounts and a stray letter is refused.
func TestAmountSuffixes(t *testing.T) {
	cases := map[string]float64{"50k": 50000, "1.2m": 1200000, "3B": 3e9, "2 k": 2000, "7": 7}
	for text, want := range cases {
		if got, ok := parseAmountWithSuffix(text); !ok || got != want {
			t.Errorf("parseAmountWithSuffix(%q) = %v, %v; want %v", text, got, ok, want)
		}
	}
	if _, ok := parseAmountWithSuffix("5x"); ok {
		t.Error("parseAmountWithSuffix(5x) accepted an unknown suffix")
	}
}

// TestJavaScriptMathHelpers checks the helpers that replace Math.round, ** and Math.log10 with JavaScript semantics.
func TestJavaScriptMathHelpers(t *testing.T) {
	roundCases := map[float64]float64{1.4: 1, 1.5: 2, -1.5: -1, -2.5: -2, 0.49999999999999994: 0}
	for in, want := range roundCases {
		if got := jsRound(in); got != want {
			t.Errorf("jsRound(%v) = %v, want %v", in, got, want)
		}
	}
	if !math.IsNaN(jsPow(1, math.Inf(1))) || !math.IsNaN(jsPow(-1, math.Inf(-1))) || !math.IsNaN(jsPow(1, math.NaN())) {
		t.Error("jsPow must yield NaN for unit bases and NaN exponents, as **")
	}
	if jsPow(2, 10) != 1024 || jsPow(0, 0) != 1 {
		t.Error("jsPow lost ordinary powers")
	}
	if log10Exact(1000) != 3 || log10Exact(1e-6) != -6 {
		t.Errorf("log10Exact on powers of ten = %v, %v", log10Exact(1000), log10Exact(1e-6))
	}
}

// TestTimespanFormatting checks seconds render as days, hours, minutes and seconds with zero parts omitted.
func TestTimespanFormatting(t *testing.T) {
	cases := map[float64]string{
		0:         "0s",
		59:        "59s",
		60:        "1m",
		8700:      "2h 25m",
		90061:     "1d 1h 1m 1s",
		-3600:     "-1h",
		86400 * 3: "3d",
		59.6:      "1m",
		-0.4:      "-0s",
	}
	for in, want := range cases {
		if got := formatTimespan(in); got != want {
			t.Errorf("formatTimespan(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestTokenizerRejectsWhatTheGrammarCannot checks the lexer errors on unknown characters and malformed numbers.
func TestTokenizerRejectsWhatTheGrammarCannot(t *testing.T) {
	for _, text := range []string{"1 ? 2", "a.b", "#", "5 é"} {
		if _, err := tokenize(text); err == nil {
			t.Errorf("tokenize(%q) accepted the text", text)
		}
	}
	tokens, err := tokenize("50k")
	if err != nil || len(tokens) != 2 || tokens[0].number != 50000 {
		t.Errorf("tokenize(50k) = %+v, %v; want one scaled number", tokens, err)
	}
	tokens, err = tokenize("1,000,000")
	if err != nil || tokens[0].number != 1000000 {
		t.Errorf("tokenize(1,000,000) = %+v, %v", tokens, err)
	}
}

// TestParserRefusesDeepNesting checks the depth cap stops hostile nesting without a stack-heavy parse.
func TestParserRefusesDeepNesting(t *testing.T) {
	deep := strings.Repeat("(", maxNestingDepth+5) + "1" + strings.Repeat(")", maxNestingDepth+5)
	if _, ok := evaluateMathExpression(deep); ok {
		t.Error("evaluateMathExpression evaluated nesting past the cap")
	}
	shallow := strings.Repeat("(", 8) + "1" + strings.Repeat(")", 8)
	if got, ok := evaluateMathExpression(shallow); !ok || got != 1 {
		t.Errorf("evaluateMathExpression(shallow) = %v, %v", got, ok)
	}
}

// TestParserPrecedenceAndAssociativity checks the grammar's operator rules, including the quirks the original keeps:
// unary minus binds tighter than power on its left, and power is right-associative.
func TestParserPrecedenceAndAssociativity(t *testing.T) {
	cases := map[string]float64{
		"2^3^2":       512,
		"-2^2":        4,
		"2+3*4":       14,
		"10-4-3":      3,
		"8/4/2":       1,
		"2*3%":        0.06,
		"100 - 10%":   90,
		"sqrt 16 + 9": 13,
		"min(4)":      4,
	}
	for text, want := range cases {
		if got, ok := evaluateMathExpression(text); !ok || math.Abs(got-want) > 1e-12 {
			t.Errorf("evaluateMathExpression(%q) = %v, %v; want %v", text, got, ok, want)
		}
	}
}

// TestEvaluationRefusesNonFiniteAndMalformed checks the evaluator never returns a value for a non-finite result or a
// broken expression.
func TestEvaluationRefusesNonFiniteAndMalformed(t *testing.T) {
	for _, text := range []string{"1/0", "0/0", "sqrt(-1)", "ln(0)", "1^(1/0)", "min()", "max(1,)", "sqrt(", "2 +", "1 2"} {
		if got, ok := evaluateMathExpression(text); ok {
			t.Errorf("evaluateMathExpression(%q) = %v, want refusal", text, got)
		}
	}
}

// TestZoneWallClockRefusesSkippedTimes checks a local time that a zone skips has no instant, and an ordinary one does.
func TestZoneWallClockRefusesSkippedTimes(t *testing.T) {
	newYork := mustLoadZone(t, "America/New_York")
	if _, ok := instantForZonedTime(newYork, 2026, time.March, 8, 2, 30); ok {
		t.Error("2:30 on the spring-forward day has an instant")
	}
	if instant, ok := instantForZonedTime(newYork, 2026, time.June, 6, 17, 0); !ok || instant.UTC().Hour() != 21 {
		t.Errorf("17:00 in New York = %v, %v; want 21:00 UTC", instant, ok)
	}
}

// TestZoneAliasesNormalize checks the alias table accepts the spellings the original accepts.
func TestZoneAliasesNormalize(t *testing.T) {
	cases := map[string]string{
		"ldn":           "Europe/London",
		"San_Francisco": "America/Los_Angeles",
		"new-york":      "America/New_York",
		"  Tokyo  ":     "Asia/Tokyo",
		"Asia/Kolkata":  "Asia/Kolkata",
	}
	var zones zoneCache
	for alias, want := range cases {
		got, _, ok := zones.zoneForAlias(alias)
		if !ok || got != want {
			t.Errorf("zoneForAlias(%q) = %q, %v; want %q", alias, got, ok, want)
		}
	}
	if _, _, ok := zones.zoneForAlias("../../etc"); ok {
		t.Error("a path-like name resolved to a zone")
	}
}

// TestMonthDayRefusesImpossibleDates checks February 30 and day 0 have no next occurrence.
func TestMonthDayRefusesImpossibleDates(t *testing.T) {
	today := time.Date(2026, time.June, 6, 0, 0, 0, 0, time.UTC)
	if _, ok := nextMonthDay(30, "feb", today, time.UTC); ok {
		t.Error("30 February resolved")
	}
	if _, ok := nextMonthDay(0, "june", today, time.UTC); ok {
		t.Error("day 0 resolved")
	}
	if got, ok := nextMonthDay(6, "June", today, time.UTC); !ok || !got.Equal(today) {
		t.Errorf("today's month day = %v, %v", got, ok)
	}
}

// TestDayArithmeticHasBounds checks a huge day count is refused, not wrapped or panicked on.
func TestDayArithmeticHasBounds(t *testing.T) {
	today := time.Date(2026, time.June, 6, 0, 0, 0, 0, time.UTC)
	if _, ok := addLocalDays(today, 1e10, time.UTC); ok {
		t.Error("ten billion days resolved")
	}
	if got, ok := addLocalDays(today, -35, time.UTC); !ok || got.Format("2006-01-02") != "2026-05-02" {
		t.Errorf("addLocalDays(-35) = %v, %v", got, ok)
	}
}

// mustLoadZone loads a named zone for a test.
func mustLoadZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc := loadZone(name)
	if loc == nil {
		t.Fatalf("zone %s did not load", name)
	}
	return loc
}
