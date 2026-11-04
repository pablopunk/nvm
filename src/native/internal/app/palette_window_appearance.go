package app

import "github.com/egoist/mygo"

// paletteAppearance is how the palette window draws its material: the material behind the pane, and whether the
// window is drawn as the active window of the app in front.
type paletteAppearance struct {
	Vibrancy         mygo.Vibrancy
	ActiveAppearance bool
}

// paletteWindowAppearance is the palette's appearance. The palette is a floating panel, which macOS draws frosted while
// it is not the key window. Drawn as active, its glass stays clear over the app in front, as the 1.x panel does.
func paletteWindowAppearance() paletteAppearance {
	return paletteAppearance{Vibrancy: mygo.VibrancyGlass, ActiveAppearance: true}
}
