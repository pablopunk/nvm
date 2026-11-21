package calc

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// fuzzSeeds are the inputs the seed corpus runs on every go test: every ported case, the currency and clock forms,
// and hostile shapes (deep nesting, huge numbers, bad unicode, empty and oversized input).
var fuzzSeeds = []string{
	"1 + 2 * 3", "(1 + 2) * 3", "2^10", "= 1 + 2", "calc 1 + 2", "calculate 1 + 2", "process.exit()", "Math.max(1,2)",
	"33-18", "33−18", "33 minus 18", "33×18", "33 divided by 3", "10--5",
	"sqrt(625)", "square root of 625", "sin(pi/2)", "min(3, 1, 2)", "max(3, 1, 2)", "round(1.4)",
	"2 power 10", "5 squared", "5 cubed", "50k + 20k", "1.2m / 3", "3b / 15", "50 k + 20 k",
	"32% of 5", "20% * 150", "150 + 20%", "200 - 15%", "50 / 10%", "19m + 47%", "19m - 47%",
	"10ft in m", "500 miles to km", "5kg in lbs", "100 c to f", "23C to F", "145 mins to timespan", "16px to rem",
	"10ft to kg", "days until 25 Dec", "December 25", "tomorrow", "35 days ago", "Monday in 3 weeks", "next Monday",
	"time in Tokyo", "5pm ldn in sf", "10 usd in gbp", "$1.2m in gbp", "8 dollars/hour in gbp", "10 $", "€10",
	"1,000 + 2,500", "calendar", "hello world", "123", "2026-08-31",
	"", " ", "=", "calc", "calc ", "=====", "((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((((1))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))",
	"sqrt(sqrt(sqrt(sqrt(sqrt(sqrt(sqrt(sqrt(sqrt(sqrt(4))))))))))", "1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1^1",
	"1e400", "1e21 + 1", "99999999999999999999999999999999999999999 k", "9999999999999999999999 + 1",
	"99999999999999 days ago", "Monday in 99999999999999 weeks", "1 / 0", "0 / 0", "0^0", "-----------1",
	"1,0000", "1,000,000.5", "1..2", "1,2,3", "5 ° in c", "°", "π", "2π", "π π", "e", "2e5",
	"time in ../../etc", "time in Europe/London", "time in ", "23:59 uk in utc", "25 pm ldn in sf",
	"10 abc in usd", "10 usd in usd", "ÿ + 1", "𝒳 + 1", "\x00 + 1", "1 + 2", "1 + 2",
	"abc(1)", "sqrt()", "min()", "max(1,)", "(1,2)", "1 2 3", "%", "%%%", "50%%", "%of", "of of of",
	strings.Repeat("9", 500), strings.Repeat("1+", 300) + "1", strings.Repeat("a", 300) + " in b",
}

// FuzzEvaluate checks that no input panics and every answer is well formed and deterministic. Under go test it runs
// the seed corpus; go test -fuzz explores beyond it.
func FuzzEvaluate(f *testing.F) {
	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}
	c := newTestCalculator(testNow)
	f.Fuzz(func(t *testing.T, input string) {
		checkEvaluateInvariants(t, c, input)
	})
}

// TestFuzzSeedCorpusHoldsInvariants runs the same invariants over the seed corpus as a plain test, so the corpus is
// checked even where fuzzing is off.
func TestFuzzSeedCorpusHoldsInvariants(t *testing.T) {
	c := newTestCalculator(testNow)
	for _, seed := range fuzzSeeds {
		checkEvaluateInvariants(t, c, seed)
	}
}

// checkEvaluateInvariants asserts properties every answer must satisfy and that repeated evaluation agrees.
func checkEvaluateInvariants(t *testing.T, c *Calculator, input string) {
	t.Helper()
	first, ok := c.Evaluate(input)
	second, again := c.Evaluate(input)
	if ok != again || !reflect.DeepEqual(first, second) {
		t.Fatalf("Evaluate(%q) is not deterministic", input)
	}
	if !ok {
		return
	}
	if math.IsNaN(first.Value) || math.IsInf(first.Value, 0) {
		t.Fatalf("Evaluate(%q).Value = %v, want finite", input, first.Value)
	}
	if first.Display == "" || first.Raw == "" || first.Kind == 0 {
		t.Fatalf("Evaluate(%q) = %+v, want display, raw and kind", input, first)
	}
	if first.Copy != first.Display {
		t.Fatalf("Evaluate(%q).Copy = %q, want the display %q", input, first.Copy, first.Display)
	}
	if !strings.HasSuffix(first.Title, " = "+first.Display) {
		t.Fatalf("Evaluate(%q).Title = %q, want it to end with the display", input, first.Title)
	}
	if len(first.Detail) == 0 || first.Detail[0].Label != "Expression" {
		t.Fatalf("Evaluate(%q).Detail = %+v, want the expression first", input, first.Detail)
	}
}
