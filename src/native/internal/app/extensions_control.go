package app

import "github.com/pablopunk/nvm/native/internal/ext/manager"

// extensionController is what the extension views ask of the manager. *manager.Manager satisfies it; tests
// substitute a recorder so a view's actions are checked without starting any child.
type extensionController interface {
	List() []manager.Listing
	Refresh() error
	Enable(id string) error
	Disable(id string) error
	Uninstall(id string) error
	Source(id string) (string, error)
}

// currentExtensionController returns the running manager as the views' controller, or nil while extensions are
// still loading. The nil interface is returned explicitly so a nil manager never reads as a live controller.
func (p *App) currentExtensionController() extensionController {
	if m := p.extensionManager(); m != nil {
		return m
	}
	return nil
}
