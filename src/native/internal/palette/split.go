package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// The palette's window sizes per presentation, as 1.x sets them (electron palette-window.ts): the default, a list
// with a side preview, and a page split beside a preview.
const (
	DefaultWidth, DefaultHeight         = 720, 470
	SidePreviewWidth, SidePreviewHeight = 960, 640
	SplitWidth, SplitHeight             = 1080, 760
)

// splitBody draws the page's body, and when the page has a side, the side beside it in an equal half of the pane, with
// a hairline between them (1.x's builder workspace is a 1fr 1fr grid). A page without a side fills the pane.
func (p *Palette) splitBody(c *ui.Context, v *View, body func()) {
	side := v.sideNow()
	if side == nil {
		body()
		return
	}
	ui.Row(c).Grow(1).FillHeight().MinHeight(0).Children(func() {
		ui.Column(c).Grow(1).FillHeight().MinHeight(0).Clip().Children(body)
		ui.Column(c).Width(1).FillHeight().Background(design.Tokens(c).EdgeSubtle).Children(func() {})
		side.draw(c, p)
	})
}

// sideNow is the side the view shows now, or nil.
func (v *View) sideNow() *Side {
	if v.Side == nil {
		return nil
	}
	return v.Side()
}

// desiredSize is the window size the top view's presentation asks for: a page split beside a preview, or a view that
// asks for a side preview or a preview, and the default size otherwise.
func (p *Palette) desiredSize() (width, height int) {
	v := p.cur()
	switch {
	case v.Presentation == SidePreview:
		return SidePreviewWidth, SidePreviewHeight
	case v.Presentation == Preview || v.sideNow() != nil:
		return SplitWidth, SplitHeight
	}
	return DefaultWidth, DefaultHeight
}

// syncPresentation resizes the window to the top view's presentation when it changed, and keeps the window placed where
// the host puts its palette. It runs on the UI thread.
func (p *Palette) syncPresentation() {
	if p.opts.Resize == nil || p.win == nil {
		return
	}
	width, height := p.desiredSize()
	if width == p.windowWidth && height == p.windowHeight {
		return
	}
	p.windowWidth, p.windowHeight = width, height
	p.opts.Resize(width, height)
	if p.opts.Position != nil {
		x, y := p.opts.Position(width, height)
		p.win.SetPosition(x, y)
	}
}
