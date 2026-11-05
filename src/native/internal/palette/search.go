package palette

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/search"
)

// The palette is the host every action runs against.
var _ search.Host = (*Palette)(nil)

// search shows the rows of the view on top for the search text. A sub-view searches on the UI thread. A view
// with providers runs them on its coordinator: rows that are ready now are shown now, and the others arrive
// through applySnapshot.
func (p *Palette) search() {
	if p.actionPending && len(p.stack) == 1 && p.actionQuery != p.query {
		p.cancelAction()
	}
	v := p.cur()
	q := strings.TrimSpace(p.query)
	if v.Providers == nil {
		var rows []search.Item
		if v.Search != nil {
			rows = v.Search(q)
		}
		p.showRows(v, q, rows, v.Loading == nil || !v.Loading(), false)
		return
	}
	if snap, ready := p.coordinator(v).Query(q, v.Providers()); ready {
		p.showRows(v, q, snap.Items, snap.Settled, snap.Pending)
	}
}

// coordinator returns the search coordinator of a view, created on its first query. Its snapshots reach the
// view through the window, and are dropped unless the view is still the top one.
func (p *Palette) coordinator(v *View) *search.Coordinator {
	if v.coord == nil {
		v.coord = search.NewCoordinator(search.Options{
			Clock:       p.opts.Clock,
			Synchronous: p.opts.Synchronous,
			Learner:     p.opts.Learner,
			Limit:       v.Limit,
			Flat:        v.Flat,
			OnSnapshot: func(s search.Snapshot) {
				p.win.Update(func() { p.applySnapshot(v, s) })
			},
		})
	}
	return v.coord
}

// applySnapshot shows a snapshot a provider produced, if the view is still on top and the snapshot is the
// generation of its query that is current.
func (p *Palette) applySnapshot(v *View, s search.Snapshot) {
	if v != p.cur() || s.Gen != v.coord.Gen() {
		return
	}
	p.showRows(v, s.Query, s.Items, s.Settled, s.Pending)
}

// showRows shows rows for a view and a query. The cursor moves to the top when the view or the query changed,
// and stays where it is when the same query is refreshed.
func (p *Palette) showRows(v *View, q string, rows []search.Item, settled, pending bool) {
	if p.shownView != v || p.shownQuery != q {
		p.chosen = 0
	}
	p.shownView, p.shownQuery = v, q
	p.results, p.settled, p.pending = rows, settled, pending
	p.chooseRequested(v)
	p.choose(p.chosen)
}

// loadingMore reports whether the rows on screen are missing a second batch that is still arriving: the providers of
// the shown query say a call is in flight, or the view's Pending hook does.
func (p *Palette) loadingMore() bool {
	v := p.cur()
	return (p.shownView == v && p.pending) || (v.Pending != nil && v.Pending()) || (v.Loading != nil && v.Loading())
}

// invalidateRows forgets the remembered rows of every view on the stack, for when the data behind them changed.
func (p *Palette) invalidateRows() {
	for _, v := range p.stack {
		if v.coord != nil {
			v.coord.Invalidate()
		}
	}
}

// chooseRequested moves the choice to the row the view asked for, once. A view whose rows change later keeps the
// choice where the user left it.
func (p *Palette) chooseRequested(v *View) {
	if v.SelectID == "" {
		return
	}
	for i, row := range p.results {
		if row.ID == v.SelectID {
			p.chosen = i
			break
		}
	}
	v.SelectID = ""
}
