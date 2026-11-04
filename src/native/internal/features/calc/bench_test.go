package calc

import "testing"

// typicalQueries are the inputs a person types most often. The budget is 50 microseconds for each.
var typicalQueries = []string{
	"1 + 2 * 3",
	"(1 + 2) * 3",
	"50k + 20k",
	"sqrt(625)",
	"32% of 5",
	"19m + 47%",
	"10ft in m",
	"100 c to f",
	"145 mins to timespan",
	"1,000 + 2,500",
	"33 divided by 3",
	"tomorrow",
	"Monday in 3 weeks",
	"time in Tokyo",
	"5pm ldn in sf",
	"calendar",
}

// BenchmarkEvaluateTypical measures one Evaluate per typical query, reported per query.
func BenchmarkEvaluateTypical(b *testing.B) {
	c := newTestCalculator(testNow)
	for _, query := range typicalQueries {
		b.Run(query, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				c.Evaluate(query)
			}
		})
	}
}

// BenchmarkQueryCachedCurrency measures a currency question answered from a fresh cached quote, the path the palette
// takes on each keystroke.
func BenchmarkQueryCachedCurrency(b *testing.B) {
	clock := clockAt(testNow)
	cache := &memCache{rates: Rates{"USD/GBP": fiatQuote(0.8, testNow)}}
	rates, err := NewRateService(cache)
	if err != nil {
		b.Fatal(err)
	}
	c := New(clock, nil, rates)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Query("10 usd in gbp")
	}
}
