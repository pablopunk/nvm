package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestPressDraggingAnExtensionRowDragsItsFile covers an extension row that declares a file, and the drag picture it
// declares when it sets one.
func TestPressDraggingAnExtensionRowDragsItsFile(t *testing.T) {
	const file = "/Users/alice/Movies/clip.mov"
	const thumb = "/Users/alice/Movies/clip.png"
	item := api.Item{ID: "clip", Title: "clip.mov", File: &api.FileRef{Path: file}, DragImage: &api.FileRef{Path: thumb}}
	row := search.Item{Title: item.Title, DragPaths: api.DragPaths(item), DragImage: api.DragImagePath(item)}
	h := newDragHarness(t, row)
	h.pressDrag("clip.mov")
	h.assertDragged(file)
	if got := h.drag.Calls()[0].Preview; got == nil {
		t.Fatal("the drag declared no picture")
	}
}
