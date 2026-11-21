package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// WindowLayout is how a palette sits in its window. The launcher is the panel; an extension window draws the view's
// title and subtitle over its page; a window whose title bar is hidden is a companion, which also starts with a strip
// that drags it, under the traffic lights.
type WindowLayout uint8

const (
	LayoutLauncher WindowLayout = iota
	LayoutWindow
	LayoutCompanion
)

// companionDragStrip is the height of the strip above the header of a companion window. It is the part of the window
// that drags it, as the 1.x companion window has.
const companionDragStrip = 36

// drawsWindowHeader reports whether the page's title and subtitle head the page. Only extension windows do.
func (p *Palette) drawsWindowHeader() bool {
	return p.opts.Layout == LayoutWindow || p.opts.Layout == LayoutCompanion
}

// dragsWindowFromStrip reports whether a strip above the header drags the window: companions have no native title bar.
func (p *Palette) dragsWindowFromStrip() bool {
	return p.opts.Layout == LayoutCompanion
}

// windowHeader is the header of an extension window: the view's title over its subtitle, as 1.x's window header is.
// The row drags the window when the title bar is hidden.
func (p *Palette) windowHeader(c *ui.Context, v *View) {
	k := design.Tokens(c)
	title := v.Title
	if v.TitleFn != nil {
		title = v.TitleFn()
	}
	if p.dragsWindowFromStrip() {
		ui.Row(c).Height(companionDragStrip).FillWidth().DragWindow()
	}
	ui.Row(c).Padding(0, 14, 6, 14).Gap(10).AlignItems(ui.Baseline).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.Primary).SingleLine().Grow(1)
		if v.Subtitle != "" {
			ui.Text(c, v.Subtitle).FontSize(12).TextColor(k.Muted).SingleLine()
		}
	})
}
