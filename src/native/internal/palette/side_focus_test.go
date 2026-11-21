package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestPreviewChangesKeepTheComposerMountedAndFocused(t *testing.T) {
	var input ui.Handle
	var side *Side
	draft := ""
	view := &View{
		Title: "Chat", Input: &input,
		Body: func(c *ui.Context, _ search.Host) {
			ui.TextAreaBase(c, &draft).Bind(&input)
		},
		Side: func() *Side { return side },
	}
	h := newSplitHarness(t, view)
	h.typeText("before")
	preview := NewSide(&View{Title: "Preview"})
	for _, step := range []struct {
		title string
		side  *Side
		text  string
	}{
		{"preview arrives", preview, " arrives"},
		{"preview hides", nil, " hides"},
		{"preview returns", preview, " returns"},
	} {
		t.Run(step.title, func(t *testing.T) {
			want := draft + step.text
			side = step.side
			h.frame()
			h.typeText(step.text)
			if draft != want {
				t.Fatalf("composer draft = %q, want %q", draft, want)
			}
		})
	}
}
