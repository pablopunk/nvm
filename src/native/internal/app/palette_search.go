package app

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search"
)

// searchClock measures the search budgets. The goldens and the benchmarks run synchronously and never wait on it.
var searchClock platform.Clock = real.NewPorts().Clock

// coordinator returns the search coordinator of a view, created on its first query. Its snapshots reach the
// view through the window, and are dropped unless the view is still the top one.
func (p *Palette) coordinator(v *View) *search.Coordinator {
	if v.coord == nil {
		v.coord = search.NewCoordinator(search.Options{
			Clock:       searchClock,
			Synchronous: p.syncSearch,
			OnSnapshot: func(s search.Snapshot) {
				p.panel.Update(func() { p.applySnapshot(v, s) })
			},
		})
	}
	return v.coord
}

// applySnapshot shows a snapshot that a provider produced, if it still belongs to the view on top and to the
// query that is current.
func (p *Palette) applySnapshot(v *View, s search.Snapshot) {
	if v != p.cur() || s.Gen != v.coord.Gen() {
		return
	}
	p.showRows(v, s.Query, s.Items, s.Settled)
}

// showRows shows rows for a view and a query. The cursor moves to the top when the view or the query changed,
// and stays where it is when the same query is refreshed, as Refresh does.
func (p *Palette) showRows(v *View, q string, rows []search.Item, settled bool) {
	if p.shownView != v || p.shownQuery != q {
		p.chosen = 0
	}
	p.shownView, p.shownQuery = v, q
	p.results, p.settled = rows, settled
	p.choose(p.chosen)
}

// invalidateRows forgets the remembered rows of every view on the stack, for when the data behind them changed.
func (p *Palette) invalidateRows() {
	for _, v := range p.stack {
		if v.coord != nil {
			v.coord.Invalidate()
		}
	}
}

// leave runs a view's OnLeave and stops its searches: nothing it returns is shown again.
func (p *Palette) leave(v *View) {
	if f := v.OnLeave; f != nil {
		f()
	}
	if v.coord != nil {
		v.coord.Cancel()
	}
}
