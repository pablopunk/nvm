package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// pointerAt is where the pointer was last seen over a row or cell, in the window's DIPs.
type pointerAt struct {
	x, y  float32
	known bool
}

// listRow draws result i as a list row. Pointer movement chooses it and a click chooses and runs it, as 1.x's cmdk
// items do. A row that drags out files draws through the host's drag element, which is its click target.
func (p *Palette) listRow(c *ui.Context, i int, draw func()) {
	it := p.results[i]
	if it.Disabled {
		draw()
		return
	}
	el := p.rowElement(c, it, draw)
	p.hoverChoose(el, i)
	p.clickRun(el, i)
}

// rowElement draws a row inside the element that takes its pointer and click: the host's drag element when the row
// drags files, else a plain column.
func (p *Palette) rowElement(c *ui.Context, it search.Item, draw func()) ui.Element {
	if src, ok := p.dragSourceOf(it); ok {
		return p.opts.DragRow(c, src, draw)
	}
	el := ui.Column(c)
	el.Children(draw)
	return el
}

// cellContent draws what a grid cell shows and returns the element that takes its click. A cell that drags files
// draws through the host's drag element, which is the click target; any other cell is clicked through the cell.
func (p *Palette) cellContent(c *ui.Context, g *Grid, cell ui.Element, i int) ui.Element {
	it := p.results[i]
	if src, ok := p.dragSourceOf(it); ok {
		return p.opts.DragRow(c, src, func() { g.Cell(c, it) })
	}
	g.Cell(c, it)
	return cell
}

// hoverChoose chooses result i when the pointer itself moved over it, as 1.x does. A list that scrolls under a still
// pointer chooses nothing, so keyboard movement is not undone. A press that is dragging chooses nothing either.
func (p *Palette) hoverChoose(el ui.Element, i int) {
	p.notePress(el)
	x, y, over := el.PointerPosition()
	if !over {
		return
	}
	b := el.Bounds()
	at := pointerAt{x: b.X + x, y: b.Y + y, known: true}
	moved := p.pointer.known && (at.x != p.pointer.x || at.y != p.pointer.y)
	if moved && !p.pressing && !el.Pressed() {
		p.choose(i)
	}
	p.pointer = at
}

// notePress records that el is pressed in this frame, so the next frame's pointer moves choose nothing.
func (p *Palette) notePress(el ui.Element) {
	if el.Pressed() {
		p.pressingNow = true
	}
}

// clickRun chooses result i and runs it when a click lands on its element. A press that moves past the drag threshold
// is no click, so a drag neither chooses nor runs.
func (p *Palette) clickRun(el ui.Element, i int) {
	if !el.Clicked() {
		return
	}
	p.clickedThisFrame = true
	p.choose(i)
	p.runDefault()
}
