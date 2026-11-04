package extitems

import (
	"fmt"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// rowBuilder turns an extension's sections into platform menu rows. invoke maps an action to the click handler the
// host runs for it, or nil when the row has nothing to run; onUI carries host-side effects to the UI thread; goos
// decides what the menu draws, as 1.x does per platform.
type rowBuilder struct {
	invoke func(action api.Action) func()
	onUI   func(fn func())
	goos   string
}

// sectionRows lays the sections out in order: a divider between sections, a disabled header for a titled section.
func (b rowBuilder) sectionRows(sections []api.MenuSection) []platform.MenuItem {
	var rows []platform.MenuItem
	for index, section := range sections {
		if index > 0 {
			rows = append(rows, platform.MenuItem{Separator: true})
		}
		if section.Title != "" {
			rows = append(rows, platform.MenuItem{ID: "section-" + strconv.Itoa(index), Label: section.Title, Disabled: true})
		}
		for position, row := range section.Rows {
			rows = append(rows, b.row(fmt.Sprintf("%d.%d", index, position), row))
		}
	}
	return rows
}

// row is one entry. A row with a submenu opens it; a row without an action is disabled.
func (b rowBuilder) row(path string, row api.MenuRow) platform.MenuItem {
	item := platform.MenuItem{
		ID:          path,
		Label:       rowLabel(row, b.goos),
		Disabled:    row.Disabled,
		Accelerator: rowAccelerator(row, b.goos),
		Icon:        menubar.MenuGlyph(string(row.Icon), b.goos),
		Checkable:   row.Checked,
		Checked:     row.Checked,
	}
	if len(row.Items) > 0 {
		for position, child := range row.Items {
			item.Submenu = append(item.Submenu, b.row(path+"."+strconv.Itoa(position), child))
		}
		return item
	}
	if row.Action == nil {
		item.Disabled = true
		return item
	}
	item.Action = b.invoke(*row.Action)
	item.Disabled = item.Action == nil
	return item
}

// rowLabel is the title, with the subtitle after an em dash. MyGo draws no second line, so the subtitle joins the label
// on every platform, as 1.x does off macOS.
func rowLabel(row api.MenuRow, goos string) string {
	if row.Subtitle == "" {
		return row.Title
	}
	return row.Title + " — " + row.Subtitle
}

// rowAccelerator is the shortcut a row shows. 1.x shows shortcuts on every platform but Linux.
func rowAccelerator(row api.MenuRow, goos string) string {
	if goos == "linux" {
		return ""
	}
	return row.Shortcut
}

// failureRows is the head of a menu that is not responding: the reason, and a row that opens the Extensions manager
// where the reason and the runtime state are listed.
func (b rowBuilder) failureRows(message string, showExtensions func()) []platform.MenuItem {
	return []platform.MenuItem{
		{ID: "failed", Label: "Not responding: " + message, Disabled: true},
		{ID: "failed-show", Label: "Show in Extensions", Action: menubar.UIAction(showExtensions, b.onUI)},
		{Separator: true},
	}
}
