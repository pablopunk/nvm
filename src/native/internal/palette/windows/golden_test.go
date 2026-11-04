package windows

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// windowGoldens is the group of the window views' goldens under testdata/golden/windows: the palette's size, in
// light and dark, at 1x and 2x.
var windowGoldens = golden.Options{Group: "windows", Width: 720, Height: 470}

func TestGoldenSwitchWindowList(t *testing.T) {
	d := newDesk(t)
	golden.AssertMatrix(t, "switch-list", goldenPalette(t, switchView(d.deps)).View, windowGoldens)
}

func TestGoldenSwitchWindowEmpty(t *testing.T) {
	d := newEmptyDesk(t)
	golden.AssertMatrix(t, "switch-empty", goldenPalette(t, switchView(d.deps)).View, windowGoldens)
}

func TestGoldenSettingsList(t *testing.T) {
	d := newDesk(t)
	golden.AssertMatrix(t, "settings", goldenPalette(t, settingsView(d.deps)).View, windowGoldens)
}

func TestGoldenSettingsNumberInput(t *testing.T) {
	d := newDesk(t)
	golden.AssertMatrix(t, "number-empty", goldenPalette(t, numberView(d.deps, settingDefByTitle(t, "Outer Gap"))).View, windowGoldens)
}

func TestGoldenSettingsNumberRefused(t *testing.T) {
	d := newDesk(t)
	p := goldenPalette(t, numberView(d.deps, settingDefByTitle(t, "Outer Gap")))
	p.SetQuery("41")
	golden.AssertMatrix(t, "number-refused", p.View, windowGoldens)
}

func TestGoldenEdgeGapFollowsOuterGap(t *testing.T) {
	d := newDesk(t)
	golden.AssertMatrix(t, "number-edge-follows", goldenPalette(t, numberView(d.deps, settingDefByTitle(t, "Top Gap"))).View, windowGoldens)
}

func TestGoldenCustomWindowCommandsNotice(t *testing.T) {
	golden.AssertMatrix(t, "custom-notice", goldenPalette(t, customView()).View, windowGoldens)
}

func TestGoldenMoveWindowFormValid(t *testing.T) {
	d := newDesk(t)
	p := goldenPalette(t, exactView(d.deps, moveForm))
	p.SetQuery("120 80")
	golden.AssertMatrix(t, "move-valid", p.View, windowGoldens)
}

func TestGoldenResizeWindowFormRefused(t *testing.T) {
	d := newDesk(t)
	p := goldenPalette(t, exactView(d.deps, resizeForm))
	p.SetQuery("0 50")
	golden.AssertMatrix(t, "resize-refused", p.View, windowGoldens)
}
