package extview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func actionTitlesOfItem(actions []api.Action) []string {
	titles := make([]string, 0, len(actions))
	for _, action := range actions {
		titles = append(titles, action.Title)
	}
	return titles
}

// TestTextRowGetsAPreviewOfItsText checks a row with text and no preview of its own gets Preview on Command-Y, and that
// the view it pushes shows the text.
func TestTextRowGetsAPreviewOfItsText(t *testing.T) {
	item := WithPreviewActions(api.Item{ID: "note", Title: "Daily note", Detail: &api.Detail{Text: "Buy milk"}})
	if got := actionTitlesOfItem(item.Actions); !slices.Equal(got, []string{api.PreviewTitle}) {
		t.Fatalf("actions = %v, want Preview only", got)
	}
	preview := item.Actions[0]
	if preview.Shortcut != api.PreviewShortcut || preview.View == nil || preview.View.Kind != api.ViewPreview || preview.View.Preview.Text != "Buy milk" {
		t.Fatalf("Preview = %+v, want a text preview on %s", preview, api.PreviewShortcut)
	}
}

// TestFileDetailGetsPreviewAndPreviewFile checks a row whose detail is a file, with no preview of its own, gets both
// options, as 1.x lists them.
func TestFileDetailGetsPreviewAndPreviewFile(t *testing.T) {
	file := api.FileRef{Path: "/tmp/a.pdf", Name: "a.pdf"}
	item := WithPreviewActions(api.Item{ID: "f", Title: "a.pdf", Detail: &api.Detail{File: &file}})
	if got := actionTitlesOfItem(item.Actions); !slices.Equal(got, []string{api.PreviewTitle, api.PreviewFileTitle}) {
		t.Fatalf("actions = %v, want Preview then Preview File", got)
	}
	if item.Actions[1].Kind != api.ActionQuickLook || item.Actions[1].Path != "/tmp/a.pdf" || item.Actions[1].Shortcut != api.PreviewShortcut {
		t.Fatalf("Preview File = %+v, want a quick look of the file on %s", item.Actions[1], api.PreviewShortcut)
	}
}

// TestRowWithItsOwnPreviewOrNothingToPreviewIsUnchanged keeps a file row's own preview alone, and adds nothing to a row
// whose File only names the file it drags, as an application row does.
func TestRowWithItsOwnPreviewOrNothingToPreviewIsUnchanged(t *testing.T) {
	file := api.FileRef{Path: "/tmp/a.txt", Name: "a.txt"}
	own := api.PreviewFile("Preview Contents", file)
	withOwn := WithPreviewActions(api.Item{ID: "f", Title: "a.txt", Detail: &api.Detail{File: &file}, Actions: []api.Action{own}})
	if got := actionTitlesOfItem(withOwn.Actions); !slices.Equal(got, []string{"Preview Contents"}) {
		t.Fatalf("actions = %v, want the row's own preview only", got)
	}
	app := WithPreviewActions(api.Item{ID: "app", Title: "Firefox", File: &file})
	if len(app.Actions) != 0 {
		t.Fatalf("actions = %v, want none for a row whose File only names its bundle", actionTitlesOfItem(app.Actions))
	}
}

// TestGenericPreviewJoinsTheActionPanelAfterItsSections checks a row whose menu is an action panel gets the generic
// options as a last section, and that the panel it came from is not changed.
func TestGenericPreviewJoinsTheActionPanelAfterItsSections(t *testing.T) {
	panel := &api.ActionPanel{Sections: []api.ActionSection{{Actions: []api.Action{{Kind: api.ActionCopyText, Title: "Copy", Text: "x"}}}}}
	item := WithPreviewActions(api.Item{ID: "n", Title: "Note", Image: &api.Image{Src: "data:image/png;base64,"}, ActionPanel: panel})
	if len(item.ActionPanel.Sections) != 2 || item.ActionPanel.Sections[1].Actions[0].Title != api.PreviewTitle {
		t.Fatalf("panel sections = %+v, want Preview in a last section", item.ActionPanel.Sections)
	}
	if len(panel.Sections) != 1 {
		t.Fatal("the caller's panel was changed")
	}
}
