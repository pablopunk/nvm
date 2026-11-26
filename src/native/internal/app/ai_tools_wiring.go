package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// extensionAITools lists and runs the AI tools enabled extensions declare, for extensions and conversation programs.
func (p *App) extensionAITools() services.AIToolHost {
	return manager.NewAIToolHost(p.aiToolSource)
}

// aiToolSource is the extension manager, or nil before the extensions start. The nil is an untyped nil interface.
func (p *App) aiToolSource() manager.AIToolSource {
	if m := p.extensionManager(); m != nil {
		return m
	}
	return nil
}
