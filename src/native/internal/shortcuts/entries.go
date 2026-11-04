package shortcuts

import (
	"cmp"
	"slices"
	"strings"
)

// PaletteID is the action ID of the palette hotkey, the row that changes the shortcut that opens Nevermind.
const PaletteID = "palette"

// PaletteTitle names the palette hotkey in its recorder, as 1.x does. The palette hotkey is not a row of the list: it
// changes in Settings.
const PaletteTitle = "Set Nevermind shortcut"

// Action is an action the user can give a shortcut: its stable ID, and the title the list shows.
type Action struct {
	ID    string
	Title string
}

// Entry is a row of the Keyboard Shortcuts list. Accelerator is "" for an action without a shortcut. Inactive says
// why a chosen shortcut is not bound. Removable is set for the user's own choices.
type Entry struct {
	ActionID    string
	Title       string
	Accelerator string
	Source      Source
	Inactive    string
	Removable   bool
}

// Entries lists every command or declared action, sorted by title. Each action shows the shortcut that applies to it:
// the user's choice, else its declared shortcut unless the user removed that.
func Entries(doc Document, commands []Action, declared []Declared, inactive map[string]string) []Entry {
	var entries []Entry
	declaredByID := map[string]Declared{}
	for _, item := range declared {
		declaredByID[item.ActionID] = item
	}
	for _, command := range commands {
		entries = append(entries, actionEntry(doc, command.ID, command.Title, declaredByID, inactive))
		delete(declaredByID, command.ID)
	}
	for _, item := range declaredByID {
		entries = append(entries, actionEntry(doc, item.ActionID, item.Title, declaredByID, inactive))
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(strings.Compare(a.Title, b.Title), strings.Compare(a.ActionID, b.ActionID))
	})
	return entries
}

func actionEntry(doc Document, actionID, title string, declared map[string]Declared, inactive map[string]string) Entry {
	entry := Entry{ActionID: actionID, Title: title, Inactive: inactive[actionID]}
	if accelerator, chosen := doc.Actions[actionID]; chosen {
		entry.Accelerator, entry.Source, entry.Removable = accelerator, SourceUser, true
		return entry
	}
	if item, ok := declared[actionID]; ok && !doc.Disabled[actionID] {
		entry.Accelerator, entry.Source = Normalize(item.Accelerator), SourceDeclared
	}
	return entry
}
