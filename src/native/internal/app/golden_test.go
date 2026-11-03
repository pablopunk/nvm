package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
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
	p.Choose(1)
	golden.AssertMatrix(t, "clipboard-preview", p.View, paletteGoldens)
}

func TestPaletteClipboardActionMenu(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.clipboardView())
	p.Choose(1)
	p.OpenActionMenu(1)
	golden.AssertMatrix(t, "clipboard-actions", p.View, paletteGoldens)
}

func TestPaletteClipboardEmptyState(t *testing.T) {
	p := goldenPalette(t)
	p.clip.Entries = nil
	goldenOpen(p, p.clipboardView())
	golden.AssertMatrix(t, "clipboard-empty", p.View, paletteGoldens)
}

func TestPaletteSnippetForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.snippetForm(nil))
	golden.AssertMatrix(t, "snippet-form", p.View, paletteGoldens)
}

func TestPaletteQuicklinkForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.quicklinkForm())
	golden.AssertMatrix(t, "quicklink-form", p.View, paletteGoldens)
}

func TestPaletteChatTranscript(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenChatView(p, false))
	golden.AssertMatrix(t, "chat-transcript", p.View, paletteGoldens)
}

// streamingGoldens lets the reply finish revealing: the fake clock moves past the reveal's catch-up.
var streamingGoldens = golden.Options{Group: "palette", Width: width, Height: height, Advance: 10 * time.Second}

func TestPaletteChatStreamingReply(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenChatView(p, true))
	golden.AssertMatrix(t, "chat-streaming", p.View, streamingGoldens)
}

func TestPaletteEmojiGridSelection(t *testing.T) {
	p := goldenPalette(t)
	view := p.emojiView()
	p.SetStack(p.rootView(), view)
	p.SetQuery("smile")
	p.emoji.refilter(p.Query())
	p.emoji.chosen = 3
	golden.AssertMatrix(t, "emoji-grid", p.View, paletteGoldens)
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
