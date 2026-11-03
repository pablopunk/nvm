package design

import "github.com/egoist/mygo/ui"

// Keycap draws a key as a small rounded cap, as macOS menus and Raycast do. The label is already spelled for
// the platform (⌘ or Ctrl), so the caller passes it through its shortcut label.
func Keycap(c *ui.Context, label string) {
	k := Tokens(c)
	ui.Row(c).Padding(1, 6).Radius(5).Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).
		Center().Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(k.Muted).SingleLine()
	})
}
