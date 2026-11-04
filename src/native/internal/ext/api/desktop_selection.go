package api

// SelectionState is the answer to Selection.Access, which never prompts.
type SelectionState string

const (
	SelectionAllowed     SelectionState = "allowed"
	SelectionDenied      SelectionState = "denied"
	SelectionUnavailable SelectionState = "unavailable"
	SelectionUnsupported SelectionState = "unsupported"
	SelectionUnknown     SelectionState = "unknown"
)

// SelectionAccess distinguishes denied permission, missing tools and unsupported sessions.
type SelectionAccess struct {
	State   SelectionState `json:"state"`
	Message string         `json:"message"`
}

// Selection reads and replaces the text selected in the app the palette was summoned from. It requires the
// desktop.selection capability. The clipboard is saved and put back by the host, and formatted clipboards (HTML, RTF,
// files, images) are refused rather than lost.
type Selection interface {
	Access() (SelectionAccess, error)
	// Text captures the selection after the palette releases focus. found is false when nothing
	// could be captured; that does not prove the source app has no selection.
	Text() (text string, found bool, err error)
	// ReplaceText pastes text over the selection of the source app. It returns false when the host could not send the
	// paste, and the text then stays on the clipboard. The paste lands in the app the selection came from.
	ReplaceText(text string) (replaced bool, err error)
	Files() ([]string, error)
}
