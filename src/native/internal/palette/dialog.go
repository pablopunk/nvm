package palette

import "github.com/pablopunk/nvm/native/internal/platform"

// GuardDialog wraps a file dialog the palette's views open. While the dialog has the keyboard the palette stays open, as
// 1.x's native dialog hold does, and when the last one closes the palette takes the keyboard back.
func (p *Palette) GuardDialog(dialog platform.FileDialog) platform.FileDialog {
	if dialog == nil {
		return nil
	}
	return guardedDialog{palette: p, dialog: dialog}
}

type guardedDialog struct {
	palette *Palette
	dialog  platform.FileDialog
}

// Choose shows the dialog and blocks until it closes. It may run off the UI thread.
func (g guardedDialog) Choose(req platform.ChooseRequest) ([]string, error) {
	g.palette.dialogs.Add(1)
	defer g.palette.dialogClosed()
	return g.dialog.Choose(req)
}

// dialogClosed records that a dialog closed, and gives the keyboard back to the palette once none is open.
func (p *Palette) dialogClosed() {
	if p.dialogs.Add(-1) != 0 || p.win == nil {
		return
	}
	p.win.Update(func() {
		if p.win.IsVisible() {
			p.win.Focus()
		}
	})
}
