// Package aliasview is the Set alias view of an action menu: the phrases that find a row in the root search.
package aliasview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Store keeps the aliases of each action.
type Store interface {
	For(actionID string) []string
	Add(actionID, alias string) error
	Remove(actionID, alias string) error
}

// Target is the row the aliases belong to: its stable action ID and its title.
type Target struct {
	ID    string
	Title string
}

// View is the Set alias view of one row. The query is the phrase to save. Each saved alias is a row that removes it when
// chosen, and Done leaves the view.
func View(target Target, store Store) *palette.View {
	return &palette.View{
		Title:       "Set alias",
		Placeholder: "Alias for “" + target.Title + "”",
		Search: func(q string) []search.Item {
			return rows(target, store, q)
		},
	}
}

func rows(target Target, store Store, query string) []search.Item {
	existing := store.For(target.ID)
	phrase := strings.TrimSpace(query)
	var out []search.Item
	if phrase != "" && !slices.Contains(existing, phrase) {
		out = append(out, search.Item{
			Title:    "Save alias “" + phrase + "”",
			Subtitle: "Make “" + target.Title + "” appear for this phrase",
			Icon:     "tag",
			Actions: []search.Action{{Title: "Save alias", Dismiss: search.DismissNever, Run: func(h search.Host) {
				saveAlias(h, store, target, phrase)
			}}},
		})
	}
	for _, alias := range existing {
		out = append(out, search.Item{
			Title:    alias,
			Subtitle: "Remove this alias",
			Icon:     "trash-2",
			Actions: []search.Action{{Title: "Remove alias", Dismiss: search.DismissNever, Run: func(h search.Host) {
				removeAlias(h, store, target, alias)
			}}},
		})
	}
	return append(out, search.Item{
		Title:    "Done",
		Subtitle: doneSubtitle(len(existing)),
		Icon:     "rotate-ccw",
		Actions:  []search.Action{{Title: "Done", Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }}},
	})
}

func saveAlias(h search.Host, store Store, target Target, phrase string) {
	if err := store.Add(target.ID, phrase); err != nil {
		h.Toast("Could not save alias: " + err.Error())
		return
	}
	h.SetQuery("")
}

func removeAlias(h search.Host, store Store, target Target, alias string) {
	if err := store.Remove(target.ID, alias); err != nil {
		h.Toast("Could not remove alias: " + err.Error())
		return
	}
	h.Refresh()
}

func doneSubtitle(saved int) string {
	switch saved {
	case 0:
		return "Close without changes"
	case 1:
		return "1 alias saved"
	}
	return strconv.Itoa(saved) + " aliases saved"
}
