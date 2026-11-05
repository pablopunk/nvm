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
// desktop.selection capability. The host snapshots all clipboard representations before capture or replacement and
// restores them while its temporary write is still owned. A later user copy is kept. A backend unable to snapshot
// rich content refuses it before changing the clipboard.
type Selection interface {
	Access() (SelectionAccess, error)
	// Text captures the selection after the palette releases focus. found is false when nothing
	// could be captured; that does not prove the source app has no selection.
	Text() (text string, found bool, err error)
	// ReplaceText pastes text over the selection of the source app. It returns false when the host could not send the
	// paste. Failed capture and paste attempts restore the original clipboard while it is still owned. The paste lands
	// in the app the selection came from.
	ReplaceText(text string) (replaced bool, err error)
	Files() ([]string, error)
}
