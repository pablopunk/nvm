package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// floatingAction is one row of the window's actions menu, opened with ⌘K: its title, its key when it has one, and
// what it runs. Only 1.x's keys are shown; Delete note has none in 1.x.
type floatingAction struct {
	title, key string
	run        func()
}

// floatingActions are the actions of the pane the window shows, in 1.x's order. The confirmation and a conflict
// have none.
func (p *App) floatingActions(edit *noteEdit) []floatingAction {
	if edit == nil {
		return nil
	}
	switch p.floatPane.pane {
	case paneEditor:
		if _, pending := edit.sess.Conflict(); pending {
			return nil
		}
		return []floatingAction{
			{title: "New note", key: "⌘N", run: p.createFloatingNote},
			{title: "All notes", key: "⌘O", run: p.showAllNotesInFloatingWindow},
			{title: "Delete note", run: p.askToDeleteFloatingNote},
		}
	case paneAllNotes:
		actions := []floatingAction{{title: "New note", key: "⌘N", run: p.createFloatingNote}}
		if selected, ok := p.floatingSelectedNote(); ok {
			actions = append(actions, floatingAction{title: "Delete note", run: func() { p.deleteNote(selected.ID) }})
		}
		return actions
	}
	return nil
}

// shownFloatingEdit is the note the floating window shows, or nil when the window is not open.
func (p *App) shownFloatingEdit() *noteEdit {
	if p.floating == nil {
		return nil
	}
	return p.floating.edit
}

// toggleFloatingActions opens or closes the actions menu, as ⌘K does, when the pane has actions.
func (p *App) toggleFloatingActions() {
	if len(p.floatingActions(p.shownFloatingEdit())) == 0 {
		return
	}
	p.floatPane.actionsOpen = !p.floatPane.actionsOpen
	p.floatPane.actionSelected = 0
}

// runFloatingAction closes the menu and runs the action chosen.
func (p *App) runFloatingAction(action floatingAction) {
	p.floatPane.actionsOpen = false
	action.run()
}

// floatingActionsKeys moves through the menu and runs its selected row.
func (p *App) floatingActionsKeys(c *ui.Context) {
	actions := p.floatingActions(p.shownFloatingEdit())
	switch {
	case c.Shortcut(0, ui.KeyDown):
		p.floatPane.actionSelected = min(p.floatPane.actionSelected+1, len(actions)-1)
	case c.Shortcut(0, ui.KeyUp):
		p.floatPane.actionSelected = max(p.floatPane.actionSelected-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		if p.floatPane.actionSelected < len(actions) {
			p.runFloatingAction(actions[p.floatPane.actionSelected])
		}
	}
}

// floatingActionsMenu draws the ⌘K menu over the window's footer: each action with its key cap, the selected one
// on the selection plate, and a click runs it.
func floatingActionsMenu(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	design.Popover(c, design.ActionPlacement, func() {
		for i, action := range p.floatingActions(edit) {
			row := ui.Row(c).Padding(6, 10).Radius(8).AlignItems(ui.Center).Gap(8)
			if i == p.floatPane.actionSelected {
				design.SelectedFill(row, k, false)
			}
			row.Children(func() {
				ui.Text(c, action.title).FontSize(13).TextColor(k.Strong).Grow(1).SingleLine()
				if action.key != "" {
					design.Keycap(c, action.key)
				}
			})
			if row.Clicked() {
				p.runFloatingAction(action)
			}
		}
	})
}

// floatingHints are the footer's key hints for the pane: Back while a menu or a confirmation is open, Open and
// Actions in the list, and Actions in the note.
func floatingHints(p *App, edit *noteEdit) []design.KeyHint {
	if p.floatPane.actionsOpen || p.floatPane.pane == paneConfirmDelete {
		return []design.KeyHint{{Title: "Back", Key: "esc"}}
	}
	if _, pending := edit.sess.Conflict(); pending && p.floatPane.pane == paneEditor {
		return nil
	}
	var hints []design.KeyHint
	if p.floatPane.pane == paneAllNotes {
		hints = append(hints, design.KeyHint{Title: "Open", Key: "↵"})
	}
	if len(p.floatingActions(edit)) > 0 {
		hints = append(hints, design.KeyHint{Title: "Actions", Key: "⌘K"})
	}
	return hints
}
