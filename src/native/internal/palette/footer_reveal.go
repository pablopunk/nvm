package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// footerRevealState remembers which selection and viewport have been revealed. Wheel scrolling
// does not change this identity, so it remains under the user's control.
type footerRevealState struct {
	selection selection
	key       string
	viewport  ui.Rect
	pending   bool
	clearance float32
}

func (p *Palette) prepareFooterReveal(c *ui.Context, list ui.Element, v *View) {
	bounds := list.Bounds()
	now := selection{v, p.query, p.chosen}
	key := ""
	if p.chosen >= 0 && p.chosen < len(p.results) {
		key = listRowKey(p.results, p.chosen).(string)
	}
	r := &p.footerReveal
	if now != r.selection || key != r.key || bounds != r.viewport {
		r.selection, r.key, r.viewport, r.pending = now, key, bounds, true
		r.clearance = p.footerOverlayClearance()
		if p.chosen >= 0 && p.chosen < len(p.results) {
			p.list.ScrollIntoView(p.rowOf(p.chosen))
		}
		c.Invalidate()
	}
}

func (p *Palette) revealFooterRow(c *ui.Context, row ui.Element) {
	r := &p.footerReveal
	bounds := row.Bounds()
	if !r.pending || bounds.H <= 0 || r.viewport.H <= 0 {
		return
	}
	bottom := r.viewport.Y + r.viewport.H - r.clearance
	if bounds.Y+bounds.H > bottom {
		index := p.chosen
		if grid := p.cur().Grid; grid != nil {
			index /= max(grid.Columns, 1)
		}
		p.list.ScrollTo(index, ui.End)
		c.Invalidate()
	}
	r.pending = false
}

// footerOverlayClearance applies only where the footer floats over the list.
func (p *Palette) footerOverlayClearance() float32 {
	if _, embedded := p.win.(sideWindow); embedded || p.cur().sideNow() != nil {
		return 0
	}
	return design.FooterClearance
}
