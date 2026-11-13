package menubar

import "github.com/pablopunk/nvm/native/internal/platform"

// UpdateRow is the menu row that replaces Check for Updates… while an update is found, downloading, installing or
// failed to install. Action is its click, and nil leaves the click doing nothing. Disabled greys the row out.
type UpdateRow struct {
	Label    string
	Action   func()
	Disabled bool
}

// menuRow is the update row as the menu draws it: the download glyph on macOS, as 1.x does.
func (u UpdateRow) menuRow(goos string, onUI func(fn func())) platform.MenuItem {
	item := row("update", u.Label, u.Action, onUI)
	item.Disabled = u.Disabled
	item.Icon = MenuGlyph("download", goos)
	return item
}
