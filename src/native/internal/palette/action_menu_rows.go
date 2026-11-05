package palette

import (
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actionMenuRows lays the menu's actions out as rows, with a header row above the first action of each titled group.
// rowAction maps each row to the index of its action, or to -1 for a header.
func actionMenuRows(acts []search.Action, chosen int) (rows []design.MenuRow, rowAction []int) {
	defaultIndex := defaultMenuIndex(acts)
	group := ""
	for i, a := range acts {
		if a.Section != "" && a.Section != group {
			rows = append(rows, design.MenuRow{Title: a.Section, Header: true})
			rowAction = append(rowAction, -1)
		}
		group = a.Section
		rows = append(rows, design.MenuRow{Title: a.Title, Key: KeyLabel(a.Label), Enter: a.Label == "" && i == defaultIndex,
			Selected: i == chosen, Disabled: a.Disabled != "", Subtitle: a.Disabled})
		rowAction = append(rowAction, i)
	}
	return rows, rowAction
}

// defaultMenuIndex is the index of the action Enter runs in the menu, or -1 when Enter runs none of them.
func defaultMenuIndex(acts []search.Action) int {
	for i, a := range acts {
		if !a.MenuOnly {
			return i
		}
	}
	return -1
}
