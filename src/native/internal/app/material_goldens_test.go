package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// materialGoldens renders the palette under the window material, as macOS shows it. A body page paints the pane surface
// where it is empty; the root list keeps the material, as the reference for lists.
var materialGoldens = golden.Options{Group: "palette", Width: width, Height: height, Interact: setMaterial}

func setMaterial(tt *ui.Tester) { tt.SetVibrancy(true) }

func TestPaletteMaterialRootList(t *testing.T) {
	p := goldenPalette(t)
	golden.AssertMatrix(t, "material-root-list", p.View, materialGoldens)
}

func TestPaletteMaterialShortcutRecorder(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	golden.AssertMatrix(t, "material-shortcut-recorder", p.View, materialGoldens)
}

func TestPaletteMaterialNoteEditor(t *testing.T) {
	p := goldenPalette(t)
	view, _ := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	golden.AssertMatrix(t, "material-note-editor", p.View, materialGoldens)
}

func TestPaletteMaterialSnippetForm(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.snippetForm(nil))
	golden.AssertMatrix(t, "material-snippet-form", p.View, materialGoldens)
}
