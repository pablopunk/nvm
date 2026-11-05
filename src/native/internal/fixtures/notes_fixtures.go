package fixtures

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// NoteConflictFixture is the view a save shows when the saved note changed under the editor: the unsaved text the
// user typed on the old version, the saved text, and the comparison Compare pushes.
func NoteConflictFixture() api.View {
	saved := "Ship the floating window\n\n- keyboard first"
	unsaved := "Ship the floating window\n\n- keyboard first\n- ship the goldens"
	savedAt := time.Date(2026, 11, 4, 9, 30, 0, 0, time.UTC)
	return api.NoteConflictView(api.Note{ID: "note-ideas", Markdown: saved, UpdatedAt: savedAt}, unsaved, "- ship the goldens (unsaved)\n\n"+saved, nil)
}
