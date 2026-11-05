package api

import "testing"

// TestNoteConflictViewOffersRestoreKeepAndCompare: the conflict view lists the two resolutions the note editor offers,
// then Compare, which pushes the preview of the two texts.
func TestNoteConflictViewOffersRestoreKeepAndCompare(t *testing.T) {
	view := NoteConflictView(Note{ID: "note-1", Markdown: "Saved text"}, "Typed text", "- Saved text", nil)
	if view.Kind != ViewList || len(view.Items) != 3 {
		t.Fatalf("view = %+v; want a list of three rows", view)
	}
	titles := []string{view.Items[0].Title, view.Items[1].Title, view.Items[2].Title}
	want := []string{"Restore unsaved changes", "Keep saved version", "Compare"}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("titles = %q; want %q", titles, want)
		}
	}
	if view.Items[0].Primary == nil || view.Items[0].Primary.Ref.ID != NoteConflictRestoreRef {
		t.Fatalf("restore row runs %+v; want %s", view.Items[0].Primary, NoteConflictRestoreRef)
	}
	if view.Items[2].Primary == nil || view.Items[2].Primary.Kind != ActionPushView {
		t.Fatalf("compare row = %+v; want a push of the preview", view.Items[2].Primary)
	}
}
