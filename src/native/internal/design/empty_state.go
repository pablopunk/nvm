package design

import "github.com/egoist/mygo/ui"

// EmptyData is what an empty result list says: a title and a line on how to fix it.
type EmptyData struct {
	Title    string
	Subtitle string
}

// EmptyState centers a search glyph with the title and subtitle in the list area. Reserve it for an active
// list with no results; passive surfaces show nothing instead.
func EmptyState(c *ui.Context, d EmptyData) {
	k := Tokens(c)
	ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
		ui.Icon(c, Icon("search")).FontSize(26).TextColor(k.Faint)
		ui.Text(c, d.Title).FontSize(15).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, d.Subtitle).FontSize(12).TextColor(k.Muted)
	})
}
