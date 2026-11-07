package app

import "github.com/pablopunk/nvm/native/internal/platform"

// displays lists the connected displays, or none when the screens port is absent.
func (p *App) displays() []platform.Display {
	if p.ports.Screens == nil {
		return nil
	}
	return p.ports.Screens.Displays()
}

// cursorWorkArea is the work area of the display under the pointer, or a fixed area when the screens port is
// absent.
func (p *App) cursorWorkArea() platform.Rect {
	if p.ports.Screens == nil {
		return platform.Rect{Width: 1280, Height: 800}
	}
	return p.ports.Screens.NearestDisplay(p.ports.Screens.CursorPoint()).WorkArea
}
