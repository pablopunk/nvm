package design

import "github.com/egoist/mygo/ui"

// HintBar draws the key hints along the bottom edge of a window, right-aligned, as 1.x's window footer does: each
// action's title, muted, with its key cap.
func HintBar(c *ui.Context, hints []KeyHint) {
	k := Tokens(c)
	ui.Row(c).Padding(8, 4, 0, 4).Gap(14).AlignItems(ui.Center).Justify(ui.End).Children(func() {
		for _, hint := range hints {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Text(c, hint.Title).FontSize(11).TextColor(k.Muted).SingleLine()
				Keycap(c, hint.Key)
			})
		}
	})
}
