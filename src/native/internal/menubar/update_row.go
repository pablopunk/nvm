package menubar

import "github.com/pablopunk/nvm/native/internal/platform"

// UpdateRow is the menu row for an update that is downloading or ready. A nil Action leaves the row as a status line.
type UpdateRow struct {
	Label  string
	Action func()
}

// RowsWithUpdate is the menu of Rows with the update row under Open, so an update is the first thing the menu offers.
// A nil row leaves the menu as Rows makes it.
func RowsWithUpdate(a Actions, onUI func(fn func()), update *UpdateRow) []platform.MenuItem {
	rows := Rows(a, onUI)
	if update == nil {
		return rows
	}
	withUpdate := make([]platform.MenuItem, 0, len(rows)+2)
	withUpdate = append(withUpdate, rows[:2]...)
	withUpdate = append(withUpdate, row("update", update.Label, update.Action, onUI), platform.MenuItem{Separator: true})
	return append(withUpdate, rows[2:]...)
}
