package app

import (
	snippetsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/quicklinks"
)

// paletteGoldens is the group of the palette goldens under testdata/golden/palette. They are the state of the
// prototype before the design package takes over styling (WP0.8): every stage after must reproduce them.
var paletteGoldens = golden.Options{Group: "palette", Width: width, Height: height}

func TestPaletteRootEmptyQuery(t *testing.T) {
	p := goldenPalette(t)
	golden.AssertMatrix(t, "root-empty", p.View, paletteGoldens)
}

func TestPaletteRootSearchSections(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "cl")
	golden.AssertMatrix(t, "root-cl", p.View, paletteGoldens)
}

func TestPaletteRootFallbackAutomate(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "show the current time in tokyo and london")
	golden.AssertMatrix(t, "root-fallback", p.View, paletteGoldens)
}

func TestPaletteClipboardPreviewSelection(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.clipboardView())
	p.Choose(0)
	golden.AssertMatrix(t, "clipboard-preview", p.View, paletteGoldens)
}

func TestPaletteClipboardActionMenu(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.clipboardView())
	p.Choose(0)
	p.OpenActionMenu(0)
	golden.AssertMatrix(t, "clipboard-actions", p.View, paletteGoldens)
}

func TestPaletteClipboardEmptyState(t *testing.T) {
	p := goldenPalette(t)
	p.clip = emptyClipboard(t)
	goldenOpen(p, p.clipboardView())
	golden.AssertMatrix(t, "clipboard-empty", p.View, paletteGoldens)
}

func TestPaletteSnippetForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, snippetsBuiltInPage(t, goldenLegacySnippets(t), snippetsbuiltin.CreateCommandID, nil))
	golden.AssertMatrix(t, "snippet-form", p.View, paletteGoldens)
}

func TestPaletteQuicklinkForm(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "create quicklink")
	itemTitled(t, p, quicklinks.CreateTitle).Actions[0].Run(p)
	p.settleExtensionCalls()
	golden.AssertMatrix(t, "quicklink-form", p.View, paletteGoldens)
}

func TestPaletteEmojiGridSelection(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.emojiView())
	goldenQuery(p, "smile")
	p.Choose(3)
	golden.AssertMatrix(t, "emoji-grid", p.View, paletteGoldens)
}

func TestPaletteEmojiNoResults(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.emojiView())
	goldenQuery(p, "zzqx")
	golden.AssertMatrix(t, "emoji-no-results", p.View, paletteGoldens)
}

func TestPaletteEmojiSkinTonePicker(t *testing.T) {
	p := goldenPalette(t)
	entry, ok := emojiCatalog.Lookup("emoji:U+1F44B")
	if !ok {
		t.Fatal("the catalog has no waving hand")
	}
	goldenOpen(p, p.skinToneView(entry))
	golden.AssertMatrix(t, "emoji-skin-tone", p.View, paletteGoldens)
}

func TestPaletteCalculatorResult(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "15% of 240 + 3*(2^5)")
	golden.AssertMatrix(t, "calc-result", p.View, paletteGoldens)
}

func TestPaletteSettingsList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.settingsView())
	golden.AssertMatrix(t, "settings", p.View, paletteGoldens)
}

func TestPaletteSearchNoResults(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.settingsView())
	goldenQuery(p, "zzqx")
	golden.AssertMatrix(t, "no-results", p.View, paletteGoldens)
}

// shellRightToLeft and shellDoubleZoom render the palette's shell in the reading direction of Arabic and at 200
// percent text: the two shell smoke tests of the validation protocol.
var (
	shellRightToLeft = golden.Options{Group: "palette", Width: width, Height: height, Interact: setRightToLeft}
	shellDoubleZoom  = golden.Options{Group: "palette", Width: width, Height: height, Interact: setDoubleZoom}
)

func setRightToLeft(tt *ui.Tester) { tt.SetDirection(ui.RightToLeft) }

func setDoubleZoom(tt *ui.Tester) { tt.SetZoom(2) }

func TestPaletteRootRightToLeft(t *testing.T) {
	p := goldenPalette(t)
	golden.AssertMatrix(t, "root-rtl", p.View, shellRightToLeft)
}

func TestPaletteRootAtDoubleZoom(t *testing.T) {
	p := goldenPalette(t)
	golden.AssertMatrix(t, "root-zoom-200", p.View, shellDoubleZoom)
}
