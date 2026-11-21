package design

import "github.com/egoist/mygo/ui"

// Keycap draws a key as a small rounded cap, as macOS menus and Raycast do. The label is already spelled for
// the platform (⌘ or Ctrl), so the caller passes it through its shortcut label.
func Keycap(c *ui.Context, label string) {
	keycapFace(c, ui.Row(c).Padding(1, 6).Radius(KeycapRadius), label)
}

// footerKeycap is a key cap inside a footer button: fully rounded, so it nests in the button's curve.
func footerKeycap(c *ui.Context, label string) {
	face := ui.Row(c).Height(footerKeyHeight).MinWidth(footerKeyHeight).Padding(0, 5).Radius(footerKeyHeight / 2)
	keycapFace(c, face, label)
}

// keycapFace gives a key cap its raised face and label.
func keycapFace(c *ui.Context, e ui.Element, label string) {
	k := Tokens(c)
	e.Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).Center().Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(k.Muted).SingleLine()
	})
}
