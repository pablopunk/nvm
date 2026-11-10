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
// still loading. The nil interface is returned explicitly so a nil manager never reads as a live controller. Every
// change the views make tells the background scheduler to bring the extension triggers up to date.
func (p *App) currentExtensionController() extensionController {
	if m := p.extensionManager(); m != nil {
		return notifyingController{extensionController: m, removed: p.removeChatsOfExtension, changed: p.extensionsChanged}
	}
	return nil
}

// notifyingController runs each change on the manager and then calls changed.
type notifyingController struct {
	extensionController
	changed func()
	// removed runs after an extension is uninstalled.
	removed func(id string)
}

func (c notifyingController) Refresh() error {
	err := c.extensionController.Refresh()
	c.changed()
	return err
}

func (c notifyingController) Enable(id string) error {
	err := c.extensionController.Enable(id)
	c.changed()
	return err
}

func (c notifyingController) Disable(id string) error {
	err := c.extensionController.Disable(id)
	c.changed()
	return err
}

func (c notifyingController) Uninstall(id string) error {
	err := c.extensionController.Uninstall(id)
	if err == nil && c.removed != nil {
		c.removed(id)
	}
	c.changed()
	return err
}
