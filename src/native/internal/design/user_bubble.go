package design

import "github.com/egoist/mygo/ui"

// UserBubble shows the user's message in a chat: right-aligned, in a lens pill.
func UserBubble(c *ui.Context, text string) {
	k := Tokens(c)
	ui.Row(c).Children(func() {
		ui.Spacer(c)
		bubble := ui.Text(c, text).FontSize(13).TextColor(k.Strong).Padding(8, ControlPad).Radius(ControlRadius).MaxWidth(520)
		Lens(bubble, k)
	})
}
