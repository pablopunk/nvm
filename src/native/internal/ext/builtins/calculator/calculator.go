// Package calculator is the built-in that answers calculator queries in the root and the palette: arithmetic,
// percentages, units, calendar and clock questions, and currency conversions priced from the cached exchange rates.
// Its answer row says the query is answered, so the web and Automate fallbacks step aside.
//
// Layer: ext. The package imports api, calc, safego, logx, search and the standard library.
package calculator

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// ID is the built-in's identity.
const ID = "nevermind.calculator"

// Extension is the calculator built-in. Its quotes live in the cache it was given, so currency answers survive a
// restart; a nil cache keeps them in memory only.
type Extension struct {
	base *calc.Calculator
	rate *calc.RateService
}

// New returns the calculator built-in over cache, the store the exchange rates are kept in. zone is where calendar
// answers are computed; nil means the machine's zone.
func New(cache calc.RateCache, zone *time.Location) Extension {
	rates, err := calc.NewRateService(cache)
	if err != nil {
		logx.Default().Warn("calculator rates could not be loaded", logx.F{"error": err.Error()})
	}
	return Extension{base: calc.New(nil, zone, rates), rate: rates}
}

// Manifest identifies the built-in. It declares the network capability because it fetches exchange rates.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Calculator"},
		Capabilities: []api.Capability{api.CapabilityNetwork},
	}
}

// SearchItems answers the query with the calculator's result row, or with none. The time is the one the call's
// context serves.
func (e Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query == "" {
		return nil, nil
	}
	engine := e.base.WithClock(ctx.Clock())
	r, ok := engine.Query(query)
	if !ok {
		e.refreshRatesSoon(ctx, query, nil)
		return nil, nil
	}
	if r.Kind == calc.KindRate {
		e.refreshRatesSoon(ctx, query, &r)
	}
	return []api.Item{resultItem(r)}, nil
}

var _ api.SearchProvider = Extension{}
