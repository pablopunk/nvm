package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	calculatorbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/calculator"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// calcQuery is the percentage-and-power query of the J03 journey: 36 + 96.
const calcQuery = "15% of 240 + 3*(2^5)"

// calcRowTitle is the title of the calculator row for calcQuery.
const calcRowTitle = "15% of 240 + 3*(2^5) = 132"

// TestCalculatorIsABuiltInExtension checks that the calculator row comes from the calculator built-in, through the
// extension manager, and not from a native root source.
func TestCalculatorIsABuiltInExtension(t *testing.T) {
	p := goldenPalette(t)
	if !p.extensionManager().IsBuiltIn(calculatorbuiltin.ID) {
		t.Fatalf("%s is not a built-in", calculatorbuiltin.ID)
	}
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, calcRowTitle)
	if row.Kind != search.KindCalculator || row.Subtitle != "Copy result to clipboard" {
		t.Fatalf("row = kind %q subtitle %q", row.Kind, row.Subtitle)
	}
}

// TestCalculatorCopyResultWritesTheResultToTheClipboard runs the first action of the row, which Enter runs, and reads
// the clipboard back.
func TestCalculatorCopyResultWritesTheResultToTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, calcRowTitle)
	row.Actions[0].Run(p)
	got, err := p.ports.Clipboard.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	if got != "132" {
		t.Fatalf("clipboard = %q, want 132", got)
	}
}

// TestCalculatorRowOffersTheActionsOf1x checks the action order of the 1.x extension: Copy Result first, then
// Continue Calculation, Copy Unformatted Result only when formatting changed the value, and Paste Result last.
func TestCalculatorRowOffersTheActionsOf1x(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, calcQuery)
	row := itemTitled(t, p, calcRowTitle)
	var titles []string
	for _, a := range row.Actions {
		titles = append(titles, a.Title)
	}
	want := []string{"Copy Result", "Continue Calculation", "Paste Result"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Fatalf("actions = %v, want %v", titles, want)
	}
}

// TestCalculatorKeysFollow1x: Continue Calculation is Command+Enter and Swap Units is Command+Shift+Enter; Paste Result
// has no key, as in 1.x. Continue puts the raw expression back in the search field and keeps the palette open.
func TestCalculatorKeysFollow1x(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "10 ft in m")
	calcRow := findResultOfKind(t, p, search.KindCalculator)
	cont := actionTitled(t, calcRow, "Continue Calculation")
	if cont.Label != palette.KeyLabel("⌃↵") || cont.Mods != mod || cont.Key != ui.KeyEnter || cont.Dismiss != search.DismissNever {
		t.Fatalf("continue = %+v", cont)
	}
	if swap := actionTitled(t, calcRow, "Swap Units"); swap.Label != palette.KeyLabel("⌃⇧↵") || swap.Mods != mod|ui.Shift || swap.Key != ui.KeyEnter {
		t.Fatalf("swap = %+v", swap)
	}
	if paste := actionTitled(t, calcRow, "Paste Result"); paste.Label != "" || paste.Key != 0 {
		t.Fatalf("paste result is bound to %q", paste.Label)
	}
	answer, ok := calc.New(fake.NewClock(), time.UTC, nil).Evaluate("10 ft in m")
	if !ok {
		t.Fatal("the calculator does not answer 10 ft in m")
	}
	cont.Run(p)
	if p.Query() != answer.Raw {
		t.Fatalf("continue left the query %q, want %q", p.Query(), answer.Raw)
	}
}

// TestCalculatorAnswerLeavesOutTheWebFallbacks checks that a calculation takes the place of the web search and
// Automate rows, and that a query the calculator does not answer keeps them.
func TestCalculatorAnswerLeavesOutTheWebFallbacks(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "2+2")
	if rows := rowTitles(p); !containsTitle(rows, "2+2 = 4") || containsPrefix(rows, "Search the web for") || containsPrefix(rows, "Automate") {
		t.Fatalf("rows for 2+2 = %v, want the calculation alone among the fallbacks", rows)
	}
	goldenQuery(p, "timer 5m")
	if rows := rowTitles(p); !containsPrefix(rows, "Search the web for") || !containsPrefix(rows, "Automate") {
		t.Fatalf("rows for timer 5m = %v, want the web fallbacks", rows)
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
	p.exts.mgr.Store(newManagerOverNetwork(t, builtInExtensions(builtInInputs{Rates: p.rateCache, Zone: time.UTC}), set.Clock, set.PublicHTTP))

	goldenQuery(p, "10 usd to eur")
	rows := p.Results()
	if len(rows) != 1 || rows[0].Kind != search.KindCalculator {
		t.Fatalf("rows = %v, want one calculator row", rowTitles(p))
	}
	if !strings.Contains(rows[0].Subtitle, "3d old") {
		t.Fatalf("subtitle = %q, want the cache age", rows[0].Subtitle)
	}
}

// TestRateRefreshStoresTheQuoteInRatesJSON searches a currency pair the cache does not hold. The calculator built-in
// fetches it through the network port in the background, and the quote is saved in the rates document, where the next
// run finds it.
func TestRateRefreshStoresTheQuoteInRatesJSON(t *testing.T) {
	p := goldenPalette(t)
	set := fake.NewSet()
	const dir = "/nvm-test/config"
	p.attachServices(set.Ports(), "linux", dir, time.UTC)
	p.exts.mgr.Store(newManagerOverNetwork(t, builtInExtensions(builtInInputs{Rates: p.rateCache, Zone: time.UTC}), set.Clock, set.PublicHTTP))
	set.PublicHTTP.Script(fake.Response{Status: 200, Body: []byte(`{"rate": 0.5, "date": "2026-01-01"}`)})

	goldenQuery(p, "10 usd to eur")
	waitForRate(t, p, "USD/EUR")
	doc, err := p.rates.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Rates["USD/EUR"].Rate; got != 0.5 {
		t.Fatalf("saved rate = %v, want 0.5", got)
	}
}

// waitForRate waits until the rates document holds pair, or fails the test. The background refresh is asynchronous.
func waitForRate(t *testing.T, p *App, pair string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if doc, err := p.rates.Load(); err == nil {
			if _, ok := doc.Rates[pair]; ok {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the rates document never held %s", pair)
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

// findResultOfKind returns the first row of the palette of kind, failing the test when there is none.
func findResultOfKind(t *testing.T, p *App, kind string) search.Item {
	t.Helper()
	for _, it := range p.Results() {
		if it.Kind == kind {
			return it
		}
	}
	t.Fatalf("no %s row among %v", kind, rowTitles(p))
	return search.Item{}
}

// containsTitle reports whether one of the titles is title.
func containsTitle(titles []string, title string) bool {
	for _, t := range titles {
		if t == title {
			return true
		}
	}
	return false
}

// containsPrefix reports whether one of the titles starts with prefix.
func containsPrefix(titles []string, prefix string) bool {
	for _, t := range titles {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}
