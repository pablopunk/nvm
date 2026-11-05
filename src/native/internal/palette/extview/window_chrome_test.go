package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// windowPalette is the palette a window of the given layout shows view in, shown on a fake window. Its page is the
// session's page, as the window host draws it.
func windowPalette(view api.View, layout palette.WindowLayout) *palette.Palette {
	page := NewSession(Deps{}, "ticker", view).Open(&fakeHost{})
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true, Layout: layout})
	p.Attach(editWindow{})
	return p
}

// TestCompanionWindowHeadsItsPageWithTitleAndSubtitle draws a companion window: the view's title and its subtitle
// head the page above its rows.
func TestCompanionWindowHeadsItsPageWithTitleAndSubtitle(t *testing.T) {
	p := windowPalette(fixtures.ExtensionWindowFixtures()[0].View, palette.LayoutCompanion)
	tt := ui.NewTester(p.View, 560, 420)
	p.Show()
	tt.Frame()
	for _, text := range []string{"Ticker", "Live quotes", "ACME 142.10"} {
		if !tt.HasText(text) {
			t.Fatalf("companion window does not draw %q", text)
		}
	}
}

// TestLauncherPanelDrawsNoWindowHeader keeps the launcher's page as it was: no subtitle line.
func TestLauncherPanelDrawsNoWindowHeader(t *testing.T) {
	p := windowPalette(fixtures.ExtensionWindowFixtures()[0].View, palette.LayoutLauncher)
	tt := ui.NewTester(p.View, 560, 420)
	p.Show()
	tt.Frame()
	if tt.HasText("Live quotes") {
		t.Fatal("launcher panel draws the window subtitle")
	}
}

// TestGoldenCompanionWindowFixture draws the floating ticker window, light and dark, at its default size.
func TestGoldenCompanionWindowFixture(t *testing.T) {
	p := windowPalette(fixtures.ExtensionWindowFixtures()[0].View, palette.LayoutCompanion)
	p.Show()
	golden.AssertMatrix(t, "companion-ticker", p.View, golden.Options{Group: "extension-window", Width: 560, Height: 420})
}
