package app

import "github.com/pablopunk/nvm/native/internal/palette"

// mod is the shortcut modifier the rows' actions show and use: ⌘ on macOS, Ctrl elsewhere. The palette owns
// it, since the palette's own keys use it too.
var mod = palette.Mod

// kb spells a label written with ⌃ for the platform.
func kb(label string) string { return palette.KeyLabel(label) }
