package api

import (
	"strings"
	"testing"
)

// TestNoteConflictViewOffersRestoreAndKeep: the conflict view lists the two resolutions the note editor offers, each
// with the preview of its note, under the 1.x title and sentence.
func TestNoteConflictViewOffersRestoreAndKeep(t *testing.T) {
	view := NoteConflictView(Note{ID: "note-1", Markdown: "Saved title\nSaved line"}, "Typed title\nTyped line", nil)
	if view.Kind != ViewList || len(view.Items) != 2 {
		t.Fatalf("view = %+v; want a list of two rows", view)
	}
	if view.Title != "Unsaved changes found" || view.Subtitle != noteConflictSubtitle || view.Placeholder != "" {
		t.Fatalf("title %q, subtitle %q, placeholder %q; want the 1.x title and sentence and no placeholder", view.Title, view.Subtitle, view.Placeholder)
	}
	titles := []string{view.Items[0].Title, view.Items[1].Title}
	want := []string{"Restore unsaved changes", "Keep saved version"}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("titles = %q; want %q", titles, want)
		}
	}
	if view.Items[0].Subtitle != "Typed line" || view.Items[1].Subtitle != "Saved line" {
		t.Fatalf("subtitles = %q, %q; want the second line of each note", view.Items[0].Subtitle, view.Items[1].Subtitle)
	}
	if view.Items[0].Primary == nil || view.Items[0].Primary.Ref.ID != NoteConflictRestoreRef {
		t.Fatalf("restore row runs %+v; want %s", view.Items[0].Primary, NoteConflictRestoreRef)
	}
	if view.Items[1].Primary == nil || view.Items[1].Primary.Ref.ID != NoteConflictKeepRef {
		t.Fatalf("keep row runs %+v; want %s", view.Items[1].Primary, NoteConflictKeepRef)
	}
}

// TestNotePreviewReadsLikeTheOneXList: the preview is the second content line without heading marks, cut to 100
// characters, and a note with one line or none reads as 1.x's fallbacks.
func TestNotePreviewReadsLikeTheOneXList(t *testing.T) {
	long := strings.Repeat("x", 120)
	cases := map[string]string{
		"":                       "Empty note",
		"\n  \n":                 "Empty note",
		"Only title":             "Markdown note",
		"# Title\r\n## Second\n": "Second",
		"Title\n" + long:         long[:100],
	}
	for markdown, want := range cases {
		if got := NotePreview(markdown); got != want {
			t.Errorf("NotePreview(%q) = %q; want %q", markdown, got, want)
		}
	}
}
