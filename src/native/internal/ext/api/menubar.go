package api

import (
	"errors"
	"fmt"
)

// maxMenuBarTitle is the visible title length next to the status glyph.
const maxMenuBarTitle = 24

// MenuBar is one status item: a native menu bar item on macOS, a tray item elsewhere. Rows that
// change often should move into Title; changing rows rebuilds the native menu.
type MenuBar struct {
	ID       string        `json:"id,omitempty"`
	Icon     MenuBarIcon   `json:"icon,omitempty"`
	Title    string        `json:"title,omitempty"`
	Tooltip  string        `json:"tooltip,omitempty"`
	Sections []MenuSection `json:"sections,omitempty"`
	// Popover opens a view under the icon on click; sections then move to right click.
	Popover *Popover `json:"popover,omitempty"`
}

// MenuSection is a titled group of rows. A title renders as a disabled header.
type MenuSection struct {
	Title string    `json:"title,omitempty"`
	Rows  []MenuRow `json:"rows"`
}

// MenuRow is one menu entry. Action runs like a global shortcut. Items makes a one-level submenu
// and replaces Action.
type MenuRow struct {
	ID       string      `json:"id"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle,omitempty"`
	Icon     MenuBarIcon `json:"icon,omitempty"`
	Checked  bool        `json:"checked,omitempty"`
	Disabled bool        `json:"disabled,omitempty"`
	// Shortcut is a display-only label such as "Command+Shift+K".
	Shortcut string    `json:"shortcut,omitempty"`
	Action   *Action   `json:"action,omitempty"`
	Items    []MenuRow `json:"items,omitempty"`
}

// Popover is a view rendered under the status item.
type Popover struct {
	View   View `json:"view"`
	Width  int  `json:"width,omitempty"`
	Height int  `json:"height,omitempty"`
}

// Validate checks the title length and the nested rows and actions. Unknown icon names are not an
// error: the host falls back to "zap" and reports a diagnostic.
func (m MenuBar) Validate() error {
	if len([]rune(m.Title)) > maxMenuBarTitle {
		return fmt.Errorf("menu bar title is longer than %d characters", maxMenuBarTitle)
	}
	for _, section := range m.Sections {
		for _, row := range section.Rows {
			if err := validateMenuRow(row, true); err != nil {
				return err
			}
		}
	}
	if m.Popover != nil && m.Sections != nil {
		return errors.New("a menu bar with a popover moves its sections to right click; set one or the other")
	}
	return nil
}

func validateMenuRow(row MenuRow, topLevel bool) error {
	if row.ID == "" || row.Title == "" {
		return errors.New("menu row needs an id and a title")
	}
	if len(row.Items) > 0 {
		if !topLevel {
			return fmt.Errorf("menu row %q: submenus nest one level only", row.ID)
		}
		if row.Action != nil {
			return fmt.Errorf("menu row %q: items replaces action; set one", row.ID)
		}
		for _, child := range row.Items {
			if err := validateMenuRow(child, false); err != nil {
				return err
			}
		}
	}
	if row.Action != nil {
		return row.Action.Validate()
	}
	return nil
}
