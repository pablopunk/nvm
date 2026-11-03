package design

import "github.com/egoist/mygo/ui"

// UserBubble shows the user's message in a chat: right-aligned, in a raised rounded pill.
func UserBubble(c *ui.Context, text string) {
	k := Tokens(c)
	ui.Row(c).Children(func() {
		ui.Spacer(c)
		ui.Text(c, text).FontSize(13).TextColor(k.Strong).Padding(8, 12).Radius(14).
			Background(k.Interactive).Border(1, k.EdgeSubtle).InsetShadow(0, 1, 0, 0, k.Hi).MaxWidth(520)
	})
}
