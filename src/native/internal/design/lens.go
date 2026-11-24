package design

import "github.com/egoist/mygo/ui"

// Lens gives a control the lens glass: the material every pill, field and bubble on the pane shares.
func Lens(e ui.Element, k *TokenSet) ui.Element {
	return e.Background(k.Lens).Border(1, k.LensEdge).InsetShadow(0, 1, 0, 0, k.LensHi).Shadow(0, 1, 3, 0, k.LensLo)
}

// GoldGlass gives the one action Enter runs the lens tinted gold.
func GoldGlass(e ui.Element, k *TokenSet) ui.Element {
	return e.Background(k.GoldGlass).Border(1, k.GoldGlassEdge).InsetShadow(0, 1, 0, 0, k.LensHi).Shadow(0, 1, 2, 0, k.LensLo)
}

// InnerPill gives a control nested in a group pill, such as a select, its plate of ink at alpha.
func InnerPill(e ui.Element, k *TokenSet) ui.Element {
	return e.Background(k.Fill).Border(1, k.LensEdge).InsetShadow(0, 1, 0, 0, k.LensHi)
}
