package snippets

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Shortcuts of a snippet's actions: copy, edit and delete. They are active while the row is selected.
const (
	copyShortcut   = "CommandOrControl+C"
	editShortcut   = "CommandOrControl+E"
	deleteShortcut = "CommandOrControl+Backspace"
)

// subtitleRunes is how much of a snippet's text a row shows on one line.
const subtitleRunes = 90

// snippetRow is the row of one snippet: its keyword and first line, and the actions. Enter pastes the text where the
// platform can send the paste gesture, and copies it where it cannot. Usage is learned under the snippet.
func snippetRow(sn Snippet, score float64, canPaste bool) api.Item {
	row := api.Item{
		ID: sn.ID, Title: sn.Name, Subtitle: rowSubtitle(sn), Icon: "type",
		Appearance: &api.Appearance{Foreground: api.ColorTeal},
		Section:    search.KindSnippets, LearnKey: "snippet:" + sn.ID, Score: score,
		Detail: snippetDetail(sn),
	}
	paste := api.Run(refPaste, pasteTitle, map[string]string{argID: sn.ID})
	copyAction := api.Run(refCopy, "Copy to Clipboard", map[string]string{argID: sn.ID})
	copyAction.Shortcut, copyAction.DismissAfterRun = copyShortcut, true
	edit := api.Run(refEdit, "Edit", map[string]string{argID: sn.ID})
	edit.Shortcut = editShortcut
	remove := api.Destructive(api.WithConfirmation(api.Run(refDelete, "Delete", map[string]string{argID: sn.ID}), api.Confirmation{
		Title: "Delete “" + sn.Name + "”", Message: "This cannot be undone.", Destructive: true,
	}))
	remove.Shortcut = deleteShortcut
	if canPaste {
		row.Primary = &paste
		row.Actions = []api.Action{copyAction, edit, remove}
	} else {
		row.Primary = &copyAction
		row.Actions = []api.Action{edit, remove}
	}
	return row
}

// rowSubtitle is the keyword, when there is one, and the first line of the text.
func rowSubtitle(sn Snippet) string {
	line := oneLine(sn.Text)
	if sn.Keyword == "" {
		return line
	}
	return sn.Keyword + "  ·  " + line
}

// oneLine collapses the text onto one line, and shortens it to what a row can show.
func oneLine(text string) string {
	line := strings.Join(strings.Fields(text), " ")
	if runes := []rune(line); len(runes) > subtitleRunes {
		return string(runes[:subtitleRunes]) + "…"
	}
	return line
}

// snippetDetail is the text of a snippet and a line on the placeholders it can use.
func snippetDetail(sn Snippet) *api.Detail {
	return &api.Detail{
		Text: sn.Text,
		Metadata: []api.Metadata{{
			Kind: api.MetadataText, Label: "Placeholders", Value: "{clipboard} and {date}", Tone: api.ToneMuted,
		}},
	}
}

// listItems are the rows of the Snippets list: the row that creates one, then every snippet by name.
func listItems(all []Snippet, canPaste bool) []api.Item {
	create := api.Run(refCreate, createTitle, nil)
	items := []api.Item{{
		ID: "create", Title: createTitle, Icon: "plus", Primary: &create,
	}}
	for _, sn := range sortedByName(all) {
		items = append(items, snippetRow(sn, 0, canPaste))
	}
	return items
}
