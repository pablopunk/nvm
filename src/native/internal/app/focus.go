package app

import "github.com/pablopunk/nvm/native/internal/platform"

// focusOption is the focus port the palette gives the keyboard back through, or nil where this OS keeps the focus
// on its own. The palette then captures nothing it could not restore.
func (p *App) focusOption() platform.Focus {
	if !p.ports.Capabilities.Supported(platform.CapabilityFocusReturn) {
		return nil
	}
	return p.ports.Focus
}
