package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/search"
)

func (p *Palette) openActionMenu(chosen int) {
	p.menuSnapshot = append([]search.Action(nil), p.menuActions()...)
	p.menuOwner = p.menuOwnerKey()
	p.actionsOpen, p.actChosen = true, chosen
	p.menuQuery = ""
	p.menuRanked = nil
	p.menuScroll = ui.ScrollState{}
	p.menuReveal = true
	p.menuPointer = pointerAt{}
	p.choicePinned = true
	p.focusList = false
	p.menuInput.Focus()
}

func (p *Palette) closeActionMenu() {
	p.resetActionMenu()
	p.focusInput()
}

func (p *Palette) filteredMenuActions() []search.Action {
	if !p.actionsOpen {
		return rankedMenuActions(p.menuActions(), p.menuQuery)
	}
	if p.menuRanked == nil || p.menuRankedQuery != p.menuQuery {
		p.menuRanked = rankedMenuActions(p.menuSnapshot, p.menuQuery)
		p.menuRankedQuery = p.menuQuery
	}
	return p.menuRanked
}

func (p *Palette) moveMenuChoice(delta int) {
	p.actChosen = max(0, min(p.actChosen+delta, len(p.filteredMenuActions())-1))
	p.menuReveal = true
}

func (p *Palette) menuChanged() { p.actChosen = 0; p.menuReveal = true }

func (p *Palette) menuTextKey(key ui.TextKey) bool {
	if key.Mods == 0 && key.Key == ui.KeyLeft && key.Text() == "" {
		p.closeActionMenu()
		return true
	}
	return false
}

func (p *Palette) actionPanel(c *ui.Context) {
	var rowAction []int
	_, height := p.desiredSize()
	if viewport := c.Root().Bounds(); viewport.H > 0 {
		height = int(viewport.H)
	}
	events := design.SearchableActionMenu(c, design.MenuSearch{
		Query: &p.menuQuery, Input: &p.menuInput, Scroll: &p.menuScroll, Reveal: &p.menuReveal,
		Changed: p.menuChanged, OnKey: p.menuTextKey, MaxHeight: float32(max(48, min(300, height-100))),
	}, func() []design.MenuRow {
		acts := p.filteredMenuActions()
		p.actChosen = min(p.actChosen, len(acts)-1)
		var rows []design.MenuRow
		rows, rowAction = actionMenuRows(acts, p.actChosen, p.menuQuery)
		return rows
	})
	p.menuReveal = false
	if events.Hovered >= 0 && rowAction[events.Hovered] >= 0 {
		pointer := pointerAt{x: events.X, y: events.Y, known: true}
		if !p.menuPointer.known || pointer.x != p.menuPointer.x || pointer.y != p.menuPointer.y {
			p.actChosen = rowAction[events.Hovered]
		}
		p.menuPointer = pointer
	}
	if events.Clicked >= 0 && rowAction[events.Clicked] >= 0 {
		p.runMenuAction(rowAction[events.Clicked])
	}
	if events.Submitted {
		p.runMenuAction(p.actChosen)
	}
}

func (p *Palette) menuOwnerKey() string {
	if row := p.chosenItem(); row != nil {
		if row.ID != "" {
			return "id:" + row.ID
		}
		return "title:" + row.Title
	}
	return ""
}

func (p *Palette) resetActionMenu() {
	p.actionsOpen = false
	p.menuQuery = ""
	p.menuSnapshot = nil
	p.menuRanked = nil
}
