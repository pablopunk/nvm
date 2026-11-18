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
	if p.actionsOpen && (p.shownView != v || p.shownQuery != q) {
		p.closeActionMenu()
	}
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
	} else if p.shownView == v {
		p.settled, p.pending = false, true
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
			Tiered:      v.Tiered,
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
	if len(s.Items) == 0 && !s.Settled {
		p.carrySupersededRows(v, s)
	}
	p.showRows(v, s.Query, s.Items, s.Settled, s.Pending)
}

// carrySupersededRows keeps on screen, for an empty unsettled snapshot of a new query, only the previous query's
// rows whose provider is still working on it, so typing does not blank the list. A provider that already answered
// has no row for this query, so its rows are cleared; showRows would otherwise keep the whole old list.
func (p *Palette) carrySupersededRows(v *View, s search.Snapshot) {
	if p.shownView == v && p.shownQuery != s.Query {
		p.results = s.Carried
	}
}

// showRows follows ranking until the user chooses a row. An explicit choice survives refresh by identity;
// changing the view or query resets that choice.
func (p *Palette) showRows(v *View, q string, rows []search.Item, settled, pending bool) {
	if i := p.defaultEnter; i != nil && (i.view != v || i.query != q || i.shows != p.shows) {
		p.defaultEnter = nil
	}
	// A previous query's retained rows are not actionable for the new query while its first snapshot is pending.
	// An empty answer for this query is authoritative and must clear rows even while other sources are pending.
	if p.shownView == v && p.shownQuery != q && len(p.results) > 0 && len(rows) == 0 && !settled {
		p.settled, p.pending = false, true
		return
	}
	newQuery := p.shownView != v || p.shownQuery != q
	if !newQuery && p.presented.view == v && p.presented.query == q {
		present := false
		if p.presented.publication != 0 {
			count := 0
			for _, row := range rows {
				if row.Provider == p.presented.provider && row.Source == p.presented.source && row.Publication == p.presented.publication {
					count++
				}
			}
			present = count == 1
		}
		if !present {
			p.presented.invalid = true
		}
	}
	before := p.chosen
	if newQuery {
		p.chosen, p.choicePinned, p.choiceLost = 0, false, false
	} else if selected := p.chosenItem(); p.choicePinned && selected != nil {
		oldIndex, newIndex := uniqueRowIndex(p.results, *selected), uniqueRowIndex(rows, *selected)
		if oldIndex >= 0 && newIndex >= 0 {
			p.chosen = newIndex
		} else {
			p.chosen, p.choicePinned, p.choiceLost = 0, false, true
		}
	} else if !p.choicePinned {
		p.chosen = 0
	}
	p.shownView, p.shownQuery = v, q
	p.rowsVersion++
	p.results, p.settled, p.pending = rows, settled, pending
	p.chooseRequested(v)
	p.clampChoice(p.chosen)
	if newQuery || p.chosen != before {
		p.scrollChoiceIntoView()
	}
	if p.actionsOpen && p.menuOwner != p.menuOwnerKey() {
		p.closeActionMenu()
	}
}

// loadingMore reports whether the rows on screen are missing a second batch that is still arriving: the providers of
// the shown query say a call is in flight, or the view's Pending hook does.
func (p *Palette) loadingMore() bool {
	v := p.cur()
	return (p.shownView == v && p.pending) || (v.Pending != nil && v.Pending()) || (v.Loading != nil && v.Loading())
}

// chooseRequested moves the choice to the row the view asked for, once. A view whose rows change later keeps the
// choice where the user left it.
func (p *Palette) chooseRequested(v *View) {
	if v.SelectID == "" {
		return
	}
	for i, row := range p.results {
		if row.ID == v.SelectID {
			p.chosen, p.choicePinned = i, true
			break
		}
	}
	v.SelectID = ""
}

// uniqueRowIndex finds an unambiguous provider-qualified ID. Anonymous rows cannot be tracked across updates.
func uniqueRowIndex(rows []search.Item, item search.Item) int {
	if item.ID == "" {
		return -1
	}
	found := -1
	for i, row := range rows {
		if row.ID == item.ID && row.Kind == item.Kind && row.Provider == item.Provider && row.Source == item.Source {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}
