package clipboard

import (
	"path/filepath"
)

// Presentation constants of the clipboard history list. Titles are the 1.x strings.
const (
	ViewID             = "clipboard-history"
	ViewTitle          = "Clipboard History"
	ViewPresentation   = "side-preview"
	ViewPlaceholder    = "Search Clipboard History"
	EmptyTitle         = "No clipboard items found."
	EmptySubtitle      = "Copy text or images and they will appear here."
	ManageSectionName  = "Manage History"
	titleRunes         = 72
	subtitleTimeLayout = "Jan 2, 3:04 PM"
)

// ActionType names what an action does. The palette maps these onto its own action kinds.
type ActionType string

// Action types of clipboard rows and menus.
const (
	ActionCopy    ActionType = "copy"
	ActionPaste   ActionType = "paste"
	ActionPreview ActionType = "preview"
	ActionPin     ActionType = "pin"
	ActionUnpin   ActionType = "unpin"
	ActionAskAI   ActionType = "ask-ai"
	ActionRemove  ActionType = "remove"
	ActionSubmenu ActionType = "submenu"
)

// Action is one row or menu action as data. An action carries an entry ID, never content, so it can cross
// the IPC boundary; the host resolves the ID when the action runs.
type Action struct {
	Type           ActionType
	Title          string
	ItemID         string
	Range          Range
	Destructive    bool
	Confirm        bool
	ConfirmLabel   string
	ConfirmMessage string
	Submenu        *Submenu
}

// Submenu is a nested list of actions. Rows share one submenu value, so a long list stays small.
type Submenu struct {
	Title   string
	Actions []Action
}

// Detail is the side preview of a row. LinkURL is set when the text is exactly one public web address.
type Detail struct {
	Title    string
	Subtitle string
	Kind     Kind
	Text     string
	ImageRef string
	Width    int
	Height   int
	LinkURL  string
}

// Row is one history entry as the palette lists it. Actions holds the preview, copy, paste, ask and pin
// actions; Manage holds the removal actions shown under "Manage History".
type Row struct {
	ID       string
	Title    string
	Subtitle string
	Kind     Kind
	Pinned   bool
	Detail   Detail
	Primary  Action
	Actions  []Action
	Manage   []Action
}

// View is the clipboard history list: rows newest first, with the history-wide removal actions.
type View struct {
	ID                string
	Title             string
	Presentation      string
	SearchPlaceholder string
	EmptyTitle        string
	EmptySubtitle     string
	Actions           []Action
	Rows              []Row
}

// BuildView returns the list of entries, shown in the given order.
func BuildView(entries []Entry, historyLen int) View {
	rows := make([]Row, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, BuildRow(e, historyLen))
	}
	return View{
		ID:                ViewID,
		Title:             ViewTitle,
		Presentation:      ViewPresentation,
		SearchPlaceholder: ViewPlaceholder,
		EmptyTitle:        EmptyTitle,
		EmptySubtitle:     EmptySubtitle,
		Actions:           RemovalActions(historyLen, nil),
		Rows:              rows,
	}
}

// BuildRow describes one entry: its title and subtitle, its detail, the copy action it runs on Enter, and
// the actions on its menu. historyLen decides whether the removal actions appear at all. The pin and ask
// actions are not in the row: RowExtraActions computes them when the action panel opens, so every row
// stays small.
func BuildRow(e Entry, historyLen int) Row {
	title := entryTitle(e)
	subtitle := entrySubtitle(e)
	copyAction := copyActionFor(e)
	actions := []Action{previewAction(e), copyAction}
	if isRichText(e) {
		actions = append(actions, Action{Type: ActionPaste, Title: "Paste Text", ItemID: e.ID})
	}
	return Row{
		ID:       "clipboard:" + e.ID,
		Title:    title,
		Subtitle: subtitle,
		Kind:     e.Kind,
		Pinned:   e.Pinned,
		Detail:   detailFor(e, title, subtitle),
		Primary:  copyAction,
		Actions:  actions,
		Manage:   RemovalActions(historyLen, &e),
	}
}

// RowExtraActions returns the actions of an entry's menu that are not part of the row: ask AI for text, and
// pin or unpin for any entry. The palette calls it when the action panel opens.
func RowExtraActions(e Entry) []Action {
	var actions []Action
	if isRichText(e) {
		actions = append(actions, Action{Type: ActionAskAI, Title: "Ask AI", ItemID: e.ID})
	}
	if e.Pinned {
		return append(actions, Action{Type: ActionUnpin, Title: "Unpin", ItemID: e.ID})
	}
	return append(actions, Action{Type: ActionPin, Title: "Pin", ItemID: e.ID})
}

// isRichText reports whether an entry holds text that can be pasted as text.
func isRichText(e Entry) bool {
	return e.Kind == KindText || e.Kind == KindHTML || e.Kind == KindRTF
}

// RemovalActions returns the removal actions for a history of historyLen entries. With an entry, they remove
// that entry or the entries of a time window. Without one, they remove the recent entries or everything.
func RemovalActions(historyLen int, e *Entry) []Action {
	if historyLen == 0 {
		return nil
	}
	if e != nil {
		return []Action{
			{
				Type:           ActionRemove,
				Title:          "Remove Item",
				ItemID:         e.ID,
				Range:          RangeItem,
				Destructive:    true,
				Confirm:        true,
				ConfirmLabel:   "Remove Item",
				ConfirmMessage: "Remove “" + entryTitle(*e) + "” from clipboard history?",
			},
			{Type: ActionSubmenu, Title: "Remove Last...", Submenu: timeRemovalSubmenu},
		}
	}
	return []Action{
		removalAction(RangeLastHour, "Remove Entries from Last Hour", "Remove clipboard history entries copied in the last hour?"),
		removalAction(RangeLastDay, "Remove Entries from Last Day", "Remove clipboard history entries copied in the last day?"),
		removalAction(RangeAll, "Remove All Entries", "Remove all clipboard history entries?"),
	}
}

// timeRemovalSubmenu is shared by every row, so the menu exists once in memory.
var timeRemovalSubmenu = &Submenu{
	Title: "Remove Last...",
	Actions: []Action{
		timeRemovalAction(RangeLast5Minutes, "5 Minutes", "the last 5 minutes"),
		timeRemovalAction(RangeLastHour, "Hour", "the last hour"),
		timeRemovalAction(RangeLastDay, "Day", "the last day"),
		timeRemovalAction(RangeLastWeek, "Week", "the last week"),
	},
}

// removalAction builds a destructive removal of a range that needs confirmation.
func removalAction(r Range, title, message string) Action {
	return Action{
		Type:           ActionRemove,
		Title:          title,
		Range:          r,
		Destructive:    true,
		Confirm:        true,
		ConfirmLabel:   title,
		ConfirmMessage: message,
	}
}

// timeRemovalAction builds a removal of a time window, confirmed with a plain "Remove".
func timeRemovalAction(r Range, title, period string) Action {
	action := removalAction(r, title, "Remove clipboard history entries copied in "+period+"?")
	action.ConfirmLabel = "Remove"
	return action
}

// previewAction opens the entry in the side preview.
func previewAction(e Entry) Action {
	return Action{Type: ActionPreview, Title: "Preview", ItemID: e.ID}
}

// copyActionFor returns the copy action of an entry, named for what it copies.
func copyActionFor(e Entry) Action {
	title := "Copy Text"
	switch e.Kind {
	case KindImage:
		title = "Copy Image"
	case KindFiles:
		title = "Copy Paths"
	}
	return Action{Type: ActionCopy, Title: title, ItemID: e.ID}
}

// entryTitle is the first line of an entry, shortened to titleRunes characters.
func entryTitle(e Entry) string {
	switch e.Kind {
	case KindImage:
		return "Clipboard image"
	case KindFiles:
		if len(e.Paths) == 0 {
			return "Clipboard files"
		}
		return filepath.Base(e.Paths[0])
	}
	runes := []rune(e.Text)
	if len(runes) > titleRunes {
		return string(runes[:titleRunes]) + "…"
	}
	return e.Text
}

// entrySubtitle names the kind and the time of the copy.
func entrySubtitle(e Entry) string {
	when := e.CreatedAt.Format(subtitleTimeLayout)
	switch e.Kind {
	case KindImage:
		return "Image copied " + when
	case KindFiles:
		return "Files copied " + when
	}
	return "Copied " + when
}

// detailFor fills the side preview of an entry.
func detailFor(e Entry, title, subtitle string) Detail {
	detail := Detail{Title: title, Subtitle: subtitle, Kind: e.Kind}
	switch e.Kind {
	case KindImage:
		detail.ImageRef = e.ImageRef
		detail.Width = e.Width
		detail.Height = e.Height
	case KindFiles:
		detail.Text = e.Text
	default:
		detail.Text = e.Text
		detail.LinkURL = SingleWebURL(e.Text)
	}
	return detail
}
