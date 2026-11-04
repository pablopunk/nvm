package app

import (
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// extensionSelection is the selected-text engine extensions use, over the app's clipboard, selection and paste ports.
// Its source is the window the palette was summoned from, so the keyboard goes back to the app the text came from.
func (p *App) extensionSelection() *selection.Engine {
	return selection.New(selection.Deps{
		Clock:        p.ports.Clock,
		Clipboard:    p.ports.Clipboard,
		SelectedText: p.ports.SelectedText,
		Paster:       p.ports.Paster,
		Focus:        p.focusOption(),
		Source:       p.summonedSource,
	})
}

// summonedSource is the window the palette was summoned from, or the zero target where none was recorded.
func (p *App) summonedSource() platform.FocusTarget {
	source, _ := p.Summoned()
	return source
}
