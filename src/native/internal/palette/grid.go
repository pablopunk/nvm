package palette

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// gridPageRows is how many grid rows Page Up and Page Down move the choice by.
const gridPageRows = 5

// gridCellRadius is the corner radius of a cell's plate.
const gridCellRadius = 8

// gridLoadingCells is how many placeholder cells follow the results while a second batch is arriving. They sit in the
// slots after the last result, so the results never move when the batch lands.
const gridLoadingCells = 3

// gridBody draws the rows of a grid view as cells, Columns to a row. The list builds only the rows on screen,
// so a grid of thousands of rows costs what its visible rows cost.
func (p *Palette) gridBody(c *ui.Context, g *Grid) {
	if it := p.chosenItem(); it != nil {
		p.announceSelection(c, it)
	}
	p.list.Selected, p.list.Selection, p.list.Key, p.list.Label = nil, nil, gridRowKey, nil
	cols := max(g.Columns, 1)
	slots := p.gridSlots(p.pendingBatch())
	rows := (slots + cols - 1) / cols
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		quiet := *c.Theme()
		quiet.Background = ui.Transparent
		quiet.Accent, quiet.AccentHover, quiet.AccentPressed = ui.Transparent, ui.Transparent, ui.Transparent
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			ui.List(c, &p.list, rows, func(r int) { p.gridRow(c, g, r, slots) }).Grow(1).FillHeight().Padding(6)
		})
	})
	if p.chosen < 0 && len(p.results) > 0 {
		p.chosen = 0
	}
}

// gridRowKey identifies a grid row. The key is a string: the automation tree reads keys as text, as the list's
// rows keys are.
func gridRowKey(row int) any {
	return strconv.Itoa(row)
}

// gridSlots is how many cells the grid lays out: one per result, and the loading placeholders while a batch arrives.
func (p *Palette) gridSlots(loading bool) int {
	if loading {
		return len(p.results) + gridLoadingCells
	}
	return len(p.results)
}

// gridRow draws one row of cells. Every row has Columns slots, so the cells of a short last row keep the columns
// of the rows above; the slots past the last result are placeholders while a batch arrives, and empty otherwise.
func (p *Palette) gridRow(c *ui.Context, g *Grid, row, slots int) {
	cols := max(g.Columns, 1)
	ui.Row(c).FillWidth().Gap(8).Padding(4, 8).Children(func() {
		for i := row * cols; i < (row+1)*cols; i++ {
			switch {
			case i < len(p.results):
				p.gridCell(c, g, i)
			case i < slots:
				p.gridLoadingCell(c, g)
			default:
				ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(g.CellHeight)
			}
		}
	})
}

// gridLoadingCell draws a placeholder cell. It has no label, plate or click: the choice never reaches it.
func (p *Palette) gridLoadingCell(c *ui.Context, g *Grid) {
	ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(g.CellHeight).Center().Children(func() { design.LoadingCell(c, g.Compact) })
}

// gridCell draws result i as a cell: the plate of the cursor or the pointer, the content Grid.Cell draws, and
// the row's title as the cell's label for assistive technology. A click chooses the cell, a double click runs it.
func (p *Palette) gridCell(c *ui.Context, g *Grid, i int) {
	k := design.Tokens(c)
	it := p.results[i]
	cell := ui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(g.CellHeight).Radius(gridCellRadius).Clip().Padding(8).Center().Label(it.Title)
	hovered := cell.Hovered()
	switch {
	case i == p.chosen:
		cell.Background(k.AccentMuted)
	case hovered:
		cell.Background(k.Selected)
	default:
		cell.Background(ui.Transparent)
	}
	clickTarget := cell
	cell.Children(func() { clickTarget = p.cellContent(c, g, cell, i) })
	p.notePress(clickTarget)
	p.hoverChoose(cell, i)
	p.clickRun(clickTarget, i)
}

// moveGridChoice moves the choice across the cells with the arrows, to the first or last cell with Home and
// End, and by a page of rows with Page Up and Page Down.
func (p *Palette) moveGridChoice(c *ui.Context, g *Grid) {
	cols := max(g.Columns, 1)
	page := gridPageRows * cols
	switch {
	case c.Shortcut(0, ui.KeyRight):
		p.choose(p.chosen + 1)
	case c.Shortcut(0, ui.KeyLeft):
		p.choose(p.chosen - 1)
	case c.Shortcut(0, ui.KeyDown):
		p.choose(p.chosen + cols)
	case c.Shortcut(0, ui.KeyUp):
		p.choose(p.chosen - cols)
	case c.Shortcut(0, ui.KeyPageDown):
		p.choose(p.chosen + page)
	case c.Shortcut(0, ui.KeyPageUp):
		p.choose(p.chosen - page)
	case c.Shortcut(0, ui.KeyHome):
		p.choose(0)
	case c.Shortcut(0, ui.KeyEnd):
		p.choose(len(p.results) - 1)
	}
}
