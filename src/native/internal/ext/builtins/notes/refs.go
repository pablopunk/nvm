package notes

// Action IDs and args the palette's goldens and tests open the built-in's views with. They name the views' own
// actions, so a test runs the same path a row or an action does.
const (
	AllNotesActionID = refAllNotes
	OpenNoteActionID = refOpenNote
	ConflictActionID = refConflict
	ArgID            = argID
	ArgMode          = argMode
	ModePalette      = modePalette
	ModeWindow       = modeWindow
)
