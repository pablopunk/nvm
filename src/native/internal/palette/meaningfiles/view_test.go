package meaningfiles

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// host records what the rows asked the palette to do. Refresh runs the search again, as the palette does.
type host struct {
	p         *palette.Palette
	refreshes int
}

func (h *host) Hide()             {}
func (h *host) Pop()              {}
func (h *host) Push(any)          {}
func (h *host) Toast(string)      {}
func (h *host) Root()             {}
func (h *host) Query() string     { return h.p.Query() }
func (h *host) SetQuery(q string) { h.p.SetQuery(q) }
func (h *host) Refresh() {
	h.refreshes++
	h.p.SetQuery(h.p.Query())
}

// fixedProvider answers every query with the same rows, as a test double for the name search or the decision.
func fixedProvider(id string, rows ...search.Item) search.Provider {
	return search.NewRowsProvider(id, func(context.Context, string) []search.Item { return rows })
}

func fileRow(id, title string) search.Item {
	return search.Item{ID: id, Kind: search.KindFiles, Title: title, Score: 75}
}

// viewWith builds the view over the providers and returns the palette that shows it, with the host that drives it.
func viewWith(t *testing.T, deps Deps) (*palette.Palette, *host) {
	t.Helper()
	root := &palette.View{Title: "Nevermind", Search: func(string) []search.Item { return nil }}
	p := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	p.SetStack(root, View(deps))
	return p, &host{p: p}
}

// titlesOf returns the row titles of the results, in order.
func titlesOf(items []search.Item) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

func askRow(t *testing.T, p *palette.Palette) search.Item {
	t.Helper()
	for _, item := range p.Results() {
		if item.ID == askRowID {
			return item
		}
	}
	t.Fatalf("no ask row in %v", titlesOf(p.Results()))
	return search.Item{}
}

func TestNameMatchesShowBeforeAnyAskAndTheAskRowIsFirst(t *testing.T) {
	p, _ := viewWith(t, Deps{
		Local:    fixedProvider("local", fileRow("/a/beach.jpg", "beach.jpg")),
		Meaning:  fixedProvider("meaning", fileRow("/a/sunset.png", "sunset.png")),
		SignedIn: func() bool { return true },
	})
	p.SetQuery("beach")
	if got := titlesOf(p.Results()); len(got) != 2 || got[0] != "Find more files with AI" || got[1] != "beach.jpg" {
		t.Fatalf("rows = %v, want the ask row then the name match", got)
	}
}

func TestMeaningMatchesJoinOnlyAfterTheUserAsks(t *testing.T) {
	p, h := viewWith(t, Deps{
		Local:    fixedProvider("local", fileRow("/a/beach.jpg", "beach.jpg")),
		Meaning:  fixedProvider("meaning", fileRow("/a/sunset.png", "sunset.png")),
		SignedIn: func() bool { return true },
	})
	p.SetQuery("holiday")
	askRow(t, p).Actions[0].Run(h)
	if h.refreshes != 1 {
		t.Fatalf("refreshes = %d, want the view refreshed once", h.refreshes)
	}
	got := titlesOf(p.Results())
	if len(got) != 2 || got[0] != "beach.jpg" || got[1] != "sunset.png" {
		t.Fatalf("rows after asking = %v, want the name match then the AI match", got)
	}
	for _, item := range p.Results() {
		if item.ID == askRowID {
			t.Fatal("the ask row is still offered for the query that was asked")
		}
	}
}

func TestANewQueryStartsAgainFromTheNameMatches(t *testing.T) {
	p, h := viewWith(t, Deps{
		Local:    fixedProvider("local", fileRow("/a/beach.jpg", "beach.jpg")),
		Meaning:  fixedProvider("meaning", fileRow("/a/sunset.png", "sunset.png")),
		SignedIn: func() bool { return true },
	})
	p.SetQuery("holiday")
	askRow(t, p).Actions[0].Run(h)
	p.SetQuery("holidays")
	for _, item := range p.Results() {
		if item.Title == "sunset.png" {
			t.Fatal("an AI match from the previous query shows for a new one")
		}
	}
	if got := askRow(t, p); got.ID != askRowID {
		t.Fatalf("ask row = %+v", got)
	}
}

func TestSignedOutViewOffersNoAskAndNoDecision(t *testing.T) {
	p, _ := viewWith(t, Deps{
		Local:    fixedProvider("local", fileRow("/a/beach.jpg", "beach.jpg")),
		Meaning:  fixedProvider("meaning", fileRow("/a/sunset.png", "sunset.png")),
		SignedIn: func() bool { return false },
	})
	p.SetQuery("beach")
	for _, item := range p.Results() {
		if item.ID == askRowID || item.Title == "sunset.png" {
			t.Fatalf("signed-out rows = %v", titlesOf(p.Results()))
		}
	}
}

func TestEmptyQueryOffersNoAsk(t *testing.T) {
	p, _ := viewWith(t, Deps{
		Local:    fixedProvider("local"),
		Meaning:  fixedProvider("meaning", fileRow("/a/sunset.png", "sunset.png")),
		SignedIn: func() bool { return true },
	})
	p.SetQuery("")
	if len(p.Results()) != 0 {
		t.Fatalf("empty query rows = %v, want none", titlesOf(p.Results()))
	}
}

func TestGatedProviderKeepsTheDecisionBudget(t *testing.T) {
	gate := gatedProvider{inner: budgetedRows{Provider: fixedProvider("meaning")}, asked: &askedQuery{}}
	if gate.ID() != "meaning" || gate.Budget().Hard != 30 {
		t.Fatalf("gate lost the inner provider's ID or budget: %s %+v", gate.ID(), gate.Budget())
	}
}

// budgetedRows is a rows provider that declares a budget, the way the decision provider does.
type budgetedRows struct {
	search.Provider
}

func (budgetedRows) Budget() search.Budget { return search.Budget{Soft: 1, Hard: 30} }
