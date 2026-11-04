package app

import (
	"context"
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// calculatorRows answers a query with the calculator's result row: its actions in the 1.x order, and its detail
// lines in a detail pane. A currency query also starts a background rate refresh; nothing here waits on the network.
func calculatorRows(p *App, _ *rootInput, q string) []search.Item {
	if q == "" {
		return nil
	}
	r, ok := p.calc.Query(q)
	if !ok {
		p.refreshRatesSoon(q, nil)
		return nil
	}
	if r.Kind == calc.KindRate {
		p.refreshRatesSoon(q, &r)
	}
	return []search.Item{calcItem(p, r)}
}

// calcItem builds the result row of one calculator answer.
func calcItem(p *App, r calc.Result) search.Item {
	actions := []search.Action{{Title: "Copy Result", Run: func(h search.Host) { p.copyText(h, r.Copy) }}}
	actions = append(actions, search.Action{Title: "Continue Calculation", Dismiss: search.DismissNever, Run: func(h search.Host) { h.SetQuery(r.Raw) }})
	if r.SwapQuery != "" {
		actions = append(actions, search.Action{Title: "Swap Units", Dismiss: search.DismissNever, Run: func(h search.Host) { h.SetQuery(r.SwapQuery) }})
	}
	if r.Raw != r.Display {
		actions = append(actions, search.Action{Title: "Copy Unformatted Result", Run: func(h search.Host) { p.copyText(h, r.Raw) }})
	}
	actions = append(actions, search.Action{Title: "Paste Result", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) { p.pasteAfterHide(h, "Paste Result", textEntry(r.Copy)) }})
	return search.Item{
		Kind: search.KindCalculator, Title: r.Title, Subtitle: r.Subtitle, Icon: "calculator", Tone: "orange",
		Score: scoreCalculator, Detail: calcDetail(r.Detail), Actions: actions,
	}
}

// calcDetail draws the detail lines of a result: a small label over its value, one pair after another.
func calcDetail(lines []calc.DetailLine) func(c *ui.Context, h search.Host) {
	return func(c *ui.Context, _ search.Host) {
		k := design.Tokens(c)
		ui.Column(c).Gap(10).Children(func() {
			for _, line := range lines {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, line.Label).FontSize(11).TextColor(k.Muted)
					ui.Text(c, line.Value).FontSize(13).TextColor(k.Strong)
				})
			}
		})
	}
}

// refreshRatesSoon fetches the rate pairs the query queued, in the background. When the answer on screen changed
// (it appeared, or its quote's age did), the palette searches again and shows it in place. shown is the answer
// on screen, nil when there was none.
func (p *App) refreshRatesSoon(q string, shown *calc.Result) {
	safego.Go("calc.rates", func(ctx context.Context) {
		if err := p.calc.Refresh(ctx); err != nil {
			log.Println("calculator rates:", err)
		}
		if p.panel == nil || !p.ratesChanged(q, shown) {
			return
		}
		p.Refresh()
	})
}

// ratesChanged reports whether the answer to q now differs from the one on screen.
func (p *App) ratesChanged(q string, shown *calc.Result) bool {
	now, ok := p.calc.Query(q)
	switch {
	case shown == nil:
		return ok
	case !ok:
		return false
	}
	return now.Display != shown.Display || now.Subtitle != shown.Subtitle
}
