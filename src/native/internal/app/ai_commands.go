package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/classifyimage"
)

// classifyImageSlot is the place of the Classify Image with AI built-in's row in the command list. Fix and Triage are
// built-in commands too, listed with their extension.
func (p *App) classifyImageSlot() []command {
	return []command{builtInSlot(classifyimage.ID, 1)}
}

// chooserUnavailable is the line shown when the operating system's chooser cannot open.
const chooserUnavailable = "The file dialog could not open"
