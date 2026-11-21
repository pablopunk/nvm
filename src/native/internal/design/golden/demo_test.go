package golden

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// demoGroup is the directory of the demo goldens, committed under testdata/golden/design-golden/.
const demoGroup = "design-golden"

// demoCard is the harness's demo view: a title, a line of body text and an accent box, in the theme's
// colors so that the dark render differs from the light one. It reads no clock and no randomness, so its
// frame is the same on every run.
func demoCard(c *ui.Context) {
	theme := c.Theme()
	ui.Column(c).Padding(16).Gap(12).Background(theme.Background).Children(func() {
		ui.Text(c, "Golden demo").FontSize(13).TextColor(theme.Text)
		ui.Text(c, "Deterministic text on paper").TextColor(theme.TextMuted)
		ui.Box(c).Size(120, 48).Radius(8).Background(theme.Accent)
	})
}

var demoOptions = Options{Group: demoGroup, Width: 240, Height: 140}

// TestDemoCardMatrix compares the demo card in light and dark at 1x and 2x with the default tolerance.
func TestDemoCardMatrix(t *testing.T) {
	AssertMatrix(t, "card", demoCard, demoOptions)
}

// TestDemoCardIsPixelExact compares the light 1x demo card exactly. It is the test that fails on a
// one-pixel change; a default comparison would pass it, since one pixel is far below 0.1 percent.
func TestDemoCardIsPixelExact(t *testing.T) {
	strict := demoOptions
	strict.Strict = true
	Assert(t, "card", demoCard, strict)
}
