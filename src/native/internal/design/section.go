package design

import "github.com/egoist/mygo/ui"

// Section draws a list's group header: the small semibold label above a run of rows.
func Section(c *ui.Context, title string) {
	k := Tokens(c)
	ui.Text(c, title).FontSize(11).FontWeight(600).TextColor(k.Muted).Padding(8, 10, 4, 10)
}
