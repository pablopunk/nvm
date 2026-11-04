package design

import "github.com/egoist/mygo/ui"

// LoadingRow draws the placeholder of a list row whose results are still arriving: a tile and two text bars in the
// rhythm of Row. It is a flat tone, not an animation, so a palette that waits on results keeps no frame requests.
func LoadingRow(c *ui.Context) {
	k := Tokens(c)
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(5, 8).Children(func() {
		ui.Row(c).Width(32).Height(32).Radius(8).Background(k.Interactive)
		ui.Column(c).Gap(6).Children(func() {
			ui.Row(c).Width(180).Height(10).Radius(4).Background(k.Interactive)
			ui.Row(c).Width(110).Height(8).Radius(4).Background(k.Interactive)
		})
	})
}

// LoadingCell draws the placeholder of a grid cell: one tile where a result's glyph would be.
func LoadingCell(c *ui.Context) {
	ui.Row(c).Width(28).Height(28).Radius(8).Background(Tokens(c).Interactive)
}
