package calc

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// testNow is the instant the TypeScript date tests use, 2026-06-06 12:00 UTC (a Saturday). Tests run in UTC so
// local-day arithmetic matches the original runs.
var testNow = time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)

// clockAt returns a fake clock moved to instant.
func clockAt(instant time.Time) *fake.Clock {
	clock := fake.NewClock()
	clock.Advance(instant.Sub(clock.Now()))
	return clock
}

// newTestCalculator returns an offline calculator whose clock reads instant and whose zone is UTC.
func newTestCalculator(instant time.Time) *Calculator {
	return New(clockAt(instant), time.UTC, nil)
}

// rawCase asserts the raw text of a query's result. An empty want asserts that the query has no result.
type rawCase struct {
	query string
	want  string
}

// checkRawCases runs each case as a subtest against the calculator.
func checkRawCases(t *testing.T, c *Calculator, cases []rawCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			got, ok := c.Evaluate(tc.query)
			if tc.want == "" {
				if ok {
					t.Fatalf("Evaluate(%q) = raw %q, want no result", tc.query, got.Raw)
				}
				return
			}
			if !ok {
				t.Fatalf("Evaluate(%q) returned no result, want raw %q", tc.query, tc.want)
			}
			if got.Raw != tc.want {
				t.Fatalf("Evaluate(%q).Raw = %q, want %q", tc.query, got.Raw, tc.want)
			}
		})
	}
}

// displayCase asserts the formatted display text of a query's result.
type displayCase struct {
	query string
	want  string
}

// checkDisplayCases runs each display case as a subtest.
func checkDisplayCases(t *testing.T, c *Calculator, cases []displayCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			got, ok := c.Evaluate(tc.query)
			if !ok {
				t.Fatalf("Evaluate(%q) returned no result, want display %q", tc.query, tc.want)
			}
			if got.Display != tc.want {
				t.Fatalf("Evaluate(%q).Display = %q, want %q", tc.query, got.Display, tc.want)
			}
		})
	}
}

// TestEvaluatesArithmeticWithoutExecutingJavaScript ports the first calculator test.
func TestEvaluatesArithmeticWithoutExecutingJavaScript(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"1 + 2 * 3", "7"},
		{"(1 + 2) * 3", "9"},
		{"2^10", "1024"},
		{"= 1 + 2", "3"},
		{"calc 1 + 2", "3"},
		{"calculate 1 + 2", "3"},
		{"process.exit()", ""},
		{"Math.max(1,2)", ""},
	})
}

// TestRecognizesCompactAndNaturalArithmeticInput ports the compact and natural arithmetic test.
func TestRecognizesCompactAndNaturalArithmeticInput(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"33-18", "15"},
		{"33−18", "15"},
		{"33 minus 18", "15"},
		{"33×18", "594"},
		{"33 divided by 3", "11"},
		{"10--5", "15"},
	})
}

// TestSupportsConstantsAndMathFunctions ports the constants and functions test.
func TestSupportsConstantsAndMathFunctions(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"sqrt(625)", "25"},
		{"square root of 625", "25"},
		{"sin(pi/2)", "1"},
		{"min(3, 1, 2)", "1"},
		{"max(3, 1, 2)", "3"},
		{"round(1.4)", "1"},
	})
}

// TestSupportsNaturalPowerPhrases ports the natural power phrase test.
func TestSupportsNaturalPowerPhrases(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"2 power 10", "1024"},
		{"4 power 6", "4096"},
		{"5 squared", "25"},
		{"5 cubed", "125"},
	})
}

// TestSupportsLargeNumberSuffixesInArithmeticContexts ports the k, m and b suffix test.
func TestSupportsLargeNumberSuffixesInArithmeticContexts(t *testing.T) {
	c := newTestCalculator(testNow)
	checkRawCases(t, c, []rawCase{
		{"50k + 20k", "70000"},
		{"1.2m / 3", "400000"},
		{"3b / 15", "200000000"},
		{"50 k + 20 k", "70000"},
	})
	checkDisplayCases(t, c, []displayCase{{"50k + 20k", "70,000"}})
}

// TestSupportsHumanPercentSemantics ports the percent test.
func TestSupportsHumanPercentSemantics(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"32% of 5", "1.6"},
		{"20% * 150", "30"},
		{"150 + 20%", "180"},
		{"200 - 15%", "170"},
		{"50 * 10%", "5"},
		{"50 / 10%", "500"},
	})
}

// TestSupportsSimpleUnitBearingPercentChanges ports the unit-bearing percent test.
func TestSupportsSimpleUnitBearingPercentChanges(t *testing.T) {
	c := newTestCalculator(testNow)
	checkRawCases(t, c, []rawCase{
		{"19m + 47%", "27.93 m"},
		{"19m - 47%", "10.07 m"},
	})
	checkDisplayCases(t, c, []displayCase{{"19m + 47%", "27.93 m"}})
}

// TestConvertsLocalOfflineUnits ports the offline unit conversion test.
func TestConvertsLocalOfflineUnits(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"10ft in m", "3.048 m"},
		{"500 miles to km", "804.672 km"},
		{"5kg in lbs", "11.0231 lb"},
		{"100 c to f", "212 °F"},
		{"23C to F", "73.4 °F"},
		{"29 inches to cm", "73.66 cm"},
		{"4 feet to cm", "121.92 cm"},
		{"3 teaspoon in ml", "14.7868 ml"},
		{"16px to rem", "1 rem"},
		{"145 mins to timespan", "2h 25m"},
	})
}

// TestKeepsConversionDimensionsStrict ports the dimension test.
func TestKeepsConversionDimensionsStrict(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"10ft to kg", ""},
		{"10kg to timespan", ""},
	})
}

// TestHandlesLocalDateAndTimezoneQuestions ports the date and timezone test, at 2026-06-06 12:00 UTC.
func TestHandlesLocalDateAndTimezoneQuestions(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"days until 25 Dec", "202 days"},
		{"December 25", "2026-12-25"},
		{"June 6", "2026-06-06"},
		{"tomorrow", "2026-06-07"},
		{"yesterday", "2026-06-05"},
		{"35 days ago", "2026-05-02"},
		{"Monday in 3 weeks", "2026-06-29"},
		{"next Monday", "2026-06-08"},
		{"time in Tokyo", "21:00"},
		{"5pm ldn in sf", "09:00"},
	})
}

// TestKeepsCompatibilityWrapperRawAndUngrouped ports the compatibility test: raw text is ungrouped and the display
// is grouped.
func TestKeepsCompatibilityWrapperRawAndUngrouped(t *testing.T) {
	c := newTestCalculator(testNow)
	checkRawCases(t, c, []rawCase{
		{"1,000 + 2,500", "3500"},
		{"10ft in m", "3.048 m"},
	})
	checkDisplayCases(t, c, []displayCase{{"1,000 + 2,500", "3,500"}})
}

// TestDoesNotTriggerForPlainNonCalculatorQueries ports the negative test: plain words, bare numbers and ISO dates
// never produce a calculator result.
func TestDoesNotTriggerForPlainNonCalculatorQueries(t *testing.T) {
	checkRawCases(t, newTestCalculator(testNow), []rawCase{
		{"calendar", ""},
		{"hello world", ""},
		{"123", ""},
		{"2026-08-31", ""},
	})
}
