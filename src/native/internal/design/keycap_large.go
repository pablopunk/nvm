package design

import "github.com/egoist/mygo/ui"

// Key radii: a menu's small key and a recorder's large key. They are named so that feature code never spells a radius.
const (
	KeycapRadius      = 5
	KeycapLargeRadius = 16
)

// KeycapLargeHeight is the side of a large key, which stands alone on a page and reads from across the room.
const KeycapLargeHeight = 84

// KeycapLarge draws a key at the size a recorder's centre stage gives it: a raised, rounded cap with the label in the
// strong ink. Use Keycap for a key set among text.
func KeycapLarge(c *ui.Context, label string) {
	k := Tokens(c)
	ui.Row(c).MinWidth(KeycapLargeHeight).Height(KeycapLargeHeight).Padding(0, 22).Radius(KeycapLargeRadius).
		Background(k.Surface).Border(1, k.EdgeStrong).InsetShadow(0, 2, 0, 0, k.Hi).Center().Children(func() {
		ui.Text(c, label).FontSize(40).FontWeight(500).TextColor(k.Strong).SingleLine()
	})
}
