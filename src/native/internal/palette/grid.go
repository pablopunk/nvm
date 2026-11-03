package palette

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// gridPageRows is how many grid rows Page Up and Page Down move the choice by.
const gridPageRows = 5

// gridCellRadius is the corner radius of a cell's plate.
const gridCellRadius = 10

// gridBody draws the rows of a grid view as cells, Columns to a row. The list builds only the rows on screen,
// so a grid of thousands of rows costs what its visible rows cost.
func (p *Palette) gridBody(c *ui.Context, g *Grid) {
	if it := p.chosenItem(); it != nil {
		p.announceSelection(c, it)
	}
	p.list.Selected, p.list.Key, p.list.Label = nil, gridRowKey, nil
	cols := max(g.Columns, 1)
	rows := (len(p.results) + cols - 1) / cols
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		quiet := *c.Theme()
		quiet.Background = ui.Transparent
		quiet.Accent, quiet.AccentHover, quiet.AccentPressed = ui.Transparent, ui.Transparent, ui.Transparent
		ui.Themed(c, &quiet).Grow(1).FillHeight().Children(func() {
			ui.List(c, &p.list, rows, func(r int) { p.gridRow(c, g, r) }).Grow(1).FillHeight().Padding(6)
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

// gridRow draws one row of cells.
func (p *Palette) gridRow(c *ui.Context, g *Grid, row int) {
	cols := max(g.Columns, 1)
	ui.Row(c).Gap(2).Padding(2, 8).Children(func() {
		for i := row * cols; i < min((row+1)*cols, len(p.results)); i++ {
			p.gridCell(c, g, i)
		}
	})
}

// gridCell draws result i as a cell: the plate of the cursor or the pointer, the content Grid.Cell draws, and
// the row's title as the cell's label for assistive technology. A click chooses the cell, a double click runs it.
func (p *Palette) gridCell(c *ui.Context, g *Grid, i int) {
	k := design.Tokens(c)
	it := p.results[i]
	cell := ui.Row(c).Width(g.CellWidth).Height(g.CellHeight).Radius(gridCellRadius).Center().Label(it.Title)
	hovered := cell.Hovered()
	switch {
	case i == p.chosen:
		cell.Background(k.AccentMuted)
	case hovered:
		cell.Background(k.Wash)
	default:
		cell.Background(ui.Transparent)
	}
	cell.Children(func() { g.Cell(c, it) })
	if cell.Clicked() {
		p.choose(i)
	}
	if cell.DoubleClicked() {
		p.runDefault()
	}
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
