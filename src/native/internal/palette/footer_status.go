package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// footer is the status bar: where you are, and what Enter and the action menu do.
func (p *Palette) footer(c *ui.Context, v *View) {
	name := v.Title
	if v.TitleFn != nil {
		name = v.TitleFn()
	}
	if v.Footer != nil {
		name = v.Footer(p.chosenItem())
	}
	root := len(p.stack) == 1
	if root {
		name = ""
	}
	loading := p.loadingActionOrView() && (len(p.results) == 0 || p.actionPending)
	if loading {
		status := "Loading…"
		if name != "" {
			name += " · "
		}
		name += status
	}
	data := design.FooterData{Name: name, HideName: root && !loading, PrimaryAction: v.FooterPrimary}
	if v.FooterControlVisible == nil || v.FooterControlVisible() {
		data.Control = v.FooterControl
	}
	if data.Control == nil && v.Accessory != nil && len(v.Accessory.Options) > 0 {
		data.Control = p.filterControl(v.Accessory)
	}
	if it := p.chosenItem(); it != nil && len(it.Actions) > 0 {
		if action, ok := it.DefaultAction(); ok {
			data.Primary = &design.KeyHint{Title: action.Title, Key: KeyLabel("↵")}
		}
		if len(p.menuActions()) > 1 {
			data.Secondary = actionsKeyHint()
		}
		if v.Primary != nil {
			data.Secondary = v.Primary()
		}
	} else if v.Body != nil && (v.Primary != nil || len(p.menuActions()) > 0) && !v.HintReplacesActions {
		p.bodyFooterKeys(v, &data)
	} else if hint := viewHint(v); hint != "" {
		data.Hint = KeyLabel(hint)
	}
	if data.Secondary != nil && data.Secondary.Title == actionsKeyHint().Title {
		data.SecondaryAction = p.openActionsFromFooter
	}
	design.Footer(c, data)
}

// bodyFooterKeys says what the primary control of a Body page does and that its action menu opens with Command-K, as
// a list's footer does for its rows.
func (p *Palette) bodyFooterKeys(v *View, data *design.FooterData) {
	if v.Primary != nil {
		data.Primary = v.Primary()
	}
	if len(p.menuActions()) > 0 {
		data.Secondary = actionsKeyHint()
	}
}

// actionsKeyHint is the footer's "Actions" entry: the key that opens the action menu.
func actionsKeyHint() *design.KeyHint {
	return &design.KeyHint{Title: "Actions", Key: KeyLabel("⌃K")}
}

// filterControl is the footer button of a view's filter: the current option, and a menu of the options to choose from.
// A view has no control in its search field; the filter lives here, beside the footer's other buttons.
func (p *Palette) filterControl(filter *Accessory) func(*ui.Context) {
	return func(c *ui.Context) {
		choice := design.Select(c, design.SelectData{Options: filter.Titles(), Selected: filter.SelectedIndex()})
		if choice >= 0 {
			filter.Choose(choice)
			p.search()
		}
	}
}
