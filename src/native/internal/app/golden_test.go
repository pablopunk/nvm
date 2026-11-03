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
	golden.AssertMatrix(t, "root-empty", p.view, paletteGoldens)
}

func TestPaletteRootSearchSections(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "cl")
	golden.AssertMatrix(t, "root-cl", p.view, paletteGoldens)
}

func TestPaletteRootFallbackAutomate(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, "show the current time in tokyo and london")
	golden.AssertMatrix(t, "root-fallback", p.view, paletteGoldens)
}

func TestPaletteClipboardPreviewSelection(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.clipboardView())
	p.choose(1)
	golden.AssertMatrix(t, "clipboard-preview", p.view, paletteGoldens)
}

func TestPaletteClipboardActionMenu(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.clipboardView())
	p.choose(1)
	p.actionsOpen, p.actChosen = true, 1
	golden.AssertMatrix(t, "clipboard-actions", p.view, paletteGoldens)
}

func TestPaletteClipboardEmptyState(t *testing.T) {
	p := goldenPalette(t)
	p.clip.Entries = nil
	goldenOpen(p, p.clipboardView())
	golden.AssertMatrix(t, "clipboard-empty", p.view, paletteGoldens)
}

func TestPaletteSnippetForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.snippetForm(nil))
	golden.AssertMatrix(t, "snippet-form", p.view, paletteGoldens)
}

func TestPaletteQuicklinkForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.quicklinkForm())
	golden.AssertMatrix(t, "quicklink-form", p.view, paletteGoldens)
}

func TestPaletteChatTranscript(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenChatView(p, false))
	golden.AssertMatrix(t, "chat-transcript", p.view, paletteGoldens)
}

// streamingGoldens lets the reply finish revealing: the fake clock moves past the reveal's catch-up.
var streamingGoldens = golden.Options{Group: "palette", Width: width, Height: height, Advance: 10 * time.Second}

func TestPaletteChatStreamingReply(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenChatView(p, true))
	golden.AssertMatrix(t, "chat-streaming", p.view, streamingGoldens)
}

func TestPaletteEmojiGridSelection(t *testing.T) {
	p := goldenPalette(t)
	view := p.emojiView()
	p.stack = []*View{p.rootView(), view}
	p.query = "smile"
	p.emoji.refilter(p.query)
	p.emoji.chosen = 3
	golden.AssertMatrix(t, "emoji-grid", p.view, paletteGoldens)
}

func TestPaletteSettingsList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.settingsView())
	golden.AssertMatrix(t, "settings", p.view, paletteGoldens)
}

func TestPaletteSearchNoResults(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.settingsView())
	goldenQuery(p, "zzqx")
	golden.AssertMatrix(t, "no-results", p.view, paletteGoldens)
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
	golden.AssertMatrix(t, "root-rtl", p.view, shellRightToLeft)
}

func TestPaletteRootAtDoubleZoom(t *testing.T) {
	p := goldenPalette(t)
	golden.AssertMatrix(t, "root-zoom-200", p.view, shellDoubleZoom)
}
