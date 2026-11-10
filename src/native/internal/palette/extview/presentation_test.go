package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// sidePreviewFixture is a list whose rows carry details, presented side-preview: the shape of an extension that shows
// the selected row's detail beside its rows (docs/extension-api.md, section 7).
func sidePreviewFixture(presentation api.Presentation) api.View {
	detail := &api.Detail{Title: "Notes", Text: "The palette keeps its window small."}
	view := listItems(api.Item{ID: "note-1", Title: "Palette notes", Detail: detail})
	view.Presentation = presentation
	return view
}

// TestSideWindowPresentationMapsToThePalette checks the wire presentations reach the palette: side-preview and preview
// are the palette's, and a list without one stacks as before.
func TestSideWindowPresentationMapsToThePalette(t *testing.T) {
	cases := []struct {
		name string
		wire api.Presentation
		want palette.Presentation
	}{
		{"side-preview", api.PresentSidePreview, palette.SidePreview},
		{"preview", api.PresentPreview, palette.Preview},
		{"unset", "", palette.Stacked},
	}
	for _, c := range cases {
		page := NewSession(Deps{}, "acme.notes", sidePreviewFixture(c.wire)).Palette()
		if page.Presentation != c.want {
			t.Errorf("%s: presentation = %v, want %v", c.name, page.Presentation, c.want)
		}
	}
}

// TestSideWindowListKeepsItsRowsAndDetail checks the side-preview list still lists its rows, so the detail of the
// chosen row has a row to belong to.
func TestSideWindowListKeepsItsRowsAndDetail(t *testing.T) {
	page := NewSession(Deps{}, "acme.notes", sidePreviewFixture(api.PresentSidePreview)).Palette()
	rows := page.Search("")
	if len(rows) != 1 || rows[0].Title != "Palette notes" || rows[0].Detail == nil {
		t.Fatalf("rows = %v, want the one row with its detail", rowTitles(rows))
	}
}
