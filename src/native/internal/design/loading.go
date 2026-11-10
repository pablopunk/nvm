package design

import "github.com/egoist/mygo/ui"

// LoadingRow follows the result row rhythm without pretending to be a selectable result.
// Static placeholders cost no animation frames and remain quiet under reduced motion.
func LoadingRow(c *ui.Context) {
	k := Tokens(c)
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Padding(8).Children(func() {
		ui.Row(c).Size(TileSize, TileSize).Radius(8).Background(k.Interactive)
		ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
			ui.Row(c).WidthPercent(48).Height(8).Radius(4).Background(k.Interactive)
			ui.Row(c).WidthPercent(30).Height(6).Radius(4).Background(k.Interactive)
		})
	})
}

// LoadingCell matches the glyph and label geometry of the grid it belongs to.
func LoadingCell(c *ui.Context, compact bool) {
	k := Tokens(c)
	ui.Column(c).FillWidth().MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Size(TileSize, TileSize).Radius(8).Background(k.Interactive)
		if !compact {
			ui.Row(c).WidthPercent(64).Height(8).Radius(4).Background(k.Interactive)
		}
	})
}
