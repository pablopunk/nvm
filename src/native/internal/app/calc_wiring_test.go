package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// calcQuery is the percentage-and-power query of the J03 journey: 36 + 96.
const calcQuery = "15% of 240 + 3*(2^5)"

// TestCalculatorRowShowsTheResultOfAPercentageQuery checks that the root shows the calculator's row for the
// canned query, with the 1.x title and the result copied by Copy Result.
func TestCalculatorRowShowsTheResultOfAPercentageQuery(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	if row.Kind != search.KindCalculator {
		t.Fatalf("kind = %q, want the calculator", row.Kind)
	}
	if row.Subtitle != "Copy result to clipboard" {
		t.Fatalf("subtitle = %q", row.Subtitle)
	}
}

// TestCalculatorCopyResultWritesTheResultToTheClipboard runs the first action of the row, which Enter runs, and
// reads the clipboard back.
func TestCalculatorCopyResultWritesTheResultToTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	row.Actions[0].Run(p)
	got, err := p.ports.Clipboard.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	if got != "132" {
		t.Fatalf("clipboard = %q, want 132", got)
	}
}

// TestCalculatorRowOffersTheOneActionsOf1x checks the action order of the 1.x extension: Copy Result first, then
// Continue Calculation, Copy Unformatted Result only when formatting changed the value, and Paste Result last.
func TestCalculatorRowOffersTheActionsOf1x(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, "15% of 240 + 3*(2^5) = 132")
	var titles []string
	for _, a := range row.Actions {
		titles = append(titles, a.Title)
	}
	want := []string{"Copy Result", "Continue Calculation", "Paste Result"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("actions = %v, want %v", titles, want)
	}
}

// TestCalculatorLabelsAStaleRateWithItsCacheAge seeds rates.json with a quote three days old, loads it through the
// app's document, and checks that the answer says how old its quote is.
func TestCalculatorLabelsAStaleRateWithItsCacheAge(t *testing.T) {
	p := goldenPalette(t)
	set := fake.NewSet()
	const dir = "/nvm-test/config"
	fetched := set.Clock.Now().Add(-72 * time.Hour)
	seedRates(t, set, dir, calc.Rates{"USD/EUR": {Rate: 0.5, Provider: "Frankfurter", UpdatedAt: fetched, FetchedAt: fetched}})
	p.attachServices(set.Ports(), "linux", dir, time.UTC)

	goldenQuery(p, "10 usd to eur")
	rows := p.Results()
	if len(rows) != 1 || rows[0].Kind != search.KindCalculator {
		t.Fatalf("rows = %v, want one calculator row", rowTitles(p))
	}
	if !strings.Contains(rows[0].Subtitle, "3d old") {
		t.Fatalf("subtitle = %q, want the cache age", rows[0].Subtitle)
	}
}

// TestRateRefreshStoresTheQuoteInRatesJSON fetches a pair through the calculator and checks that the quote is saved
// in the rates document, where the next run finds it.
func TestRateRefreshStoresTheQuoteInRatesJSON(t *testing.T) {
	p := goldenPalette(t)
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: 200, Body: []byte(`{"rate": 0.5, "date": "2026-01-01"}`)})
	const dir = "/nvm-test/config"
	p.attachServices(set.Ports(), "linux", dir, time.UTC)

	if _, ok := p.calc.Query("10 usd to eur"); ok {
		t.Fatal("a quote answered before any was fetched")
	}
	if err := p.calc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	doc, err := p.rates.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Rates["USD/EUR"].Rate; got != 0.5 {
		t.Fatalf("saved rate = %v, want 0.5", got)
	}
}

// seedRates writes rates.json into dir on set's file system, as an earlier run of the app would have.
func seedRates(t *testing.T, set *fake.Set, dir string, rates calc.Rates) {
	t.Helper()
	st, err := store.Open(set.FS, set.Clock, dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := store.Doc(st, "rates.json", 1, defaultRateDocument)
	if err := doc.Update(func(v *rateDocument) { v.Rates = rates }); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}
