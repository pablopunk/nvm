package palette

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/design"
)

// coldLoading reserves a useful preview of the result layout. It owns no focus, selection,
// clocks or input handlers; cached results always use the ordinary interactive surface.
func (p *Palette) coldLoading(c *ui.Context, g *Grid) {
	ui.Column(c).Grow(1).MinHeight(0).Clip().Padding(8).Children(func() {
		if g == nil {
			for range 7 {
				design.LoadingRow(c)
			}
			return
		}
		rows := 3
		if g.Compact {
			rows = 6
		}
		for range rows {
			ui.Row(c).FillWidth().Gap(8).Padding(4, 8).Children(func() {
				for range max(g.Columns, 1) {
					ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(g.CellHeight).Center().Children(func() {
						design.LoadingCell(c, g.Compact)
					})
				}
			})
		}
	})
}

// pendingBatch reserves appended placeholders only for incremental results, never a whole-view refresh.
func (p *Palette) pendingBatch() bool {
	v := p.cur()
	return (p.shownView == v && p.pending) || (v.Pending != nil && v.Pending())
}
