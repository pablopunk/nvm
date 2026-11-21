package app

import "github.com/egoist/mygo"

// applyIndicatorAppearance keeps non-key status panels on the same clear system glass as the palette.
func applyIndicatorAppearance(window *mygo.Window) {
	appearance := paletteWindowAppearance()
	window.SetVibrancy(appearance.Vibrancy)
	window.SetActiveAppearance(appearance.ActiveAppearance)
}
