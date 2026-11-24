package design

import "github.com/egoist/mygo/ui"

// Keycap draws a key as a small rounded cap, as macOS menus and Raycast do. The label is already spelled for
// the platform (⌘ or Ctrl), so the caller passes it through its shortcut label.
func Keycap(c *ui.Context, label string) {
	keycapFace(c, ui.Row(c).Padding(1, 6).Radius(KeycapRadius), label, Tokens(c).Muted)
}

// footerKeycap is a key cap inside a footer button: fully rounded, so it nests in the button's curve, in the button's ink.
func footerKeycap(c *ui.Context, label string, ink ui.Color) {
	face := ui.Row(c).Height(KeyHeight).MinWidth(KeyHeight).Padding(0, 5).Radius(KeyRadius)
	keycapFace(c, face, label, ink)
}

// keycapFace raises a key cap out of the control it sits on. It has no surface of its own: its plate, edge and drop are
// the ink of that control at alpha and its glyph the ink itself, so it reads on a lens, on gold glass and on ink alike.
func keycapFace(c *ui.Context, e ui.Element, label string, ink ui.Color) {
	k := Tokens(c)
	e.Background(ink.Alpha(0.12)).Border(1, ink.Alpha(0.2)).InsetShadow(0, 1, 0, 0, k.KeyHi).
		Shadow(0, 1, 1.5, 0, ink.Alpha(0.3)).Center().Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(ink).SingleLine()
	})
}
