package app

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// extensionsEditor is the host port of the extensions service, for extensions and conversation programs. It turns
// extensions off and shows or hides their menu bar items, but it never enables one: OpenReview opens the same review
// the Extensions list opens, and the user enables the extension there.
type extensionsEditor struct {
	controller func() extensionController
	builtIn    func(id string) bool
	menuBars   func() []manager.MenuBarExtension
	visibility *menuBarVisibility
	onUI       func(func())
	// openReview shows the review of a listing in the palette. It runs on the UI thread.
	openReview func(ctl extensionController, l manager.Listing)
	// menuBarChanged brings the menu bar items in line with the visibility. It runs on the UI thread.
	menuBarChanged func()
}

func (p *App) extensionsEditor() services.ExtensionsEditor {
	return extensionsEditor{
		controller: p.currentExtensionController, builtIn: p.extensionIsBuiltIn, menuBars: p.runningMenuBarExtensions,
		visibility: &p.visibility, onUI: p.onUI, openReview: p.openExtensionReview, menuBarChanged: p.syncMenuBarItems,
	}
}

// List is every folder extension, then the built-ins that have a menu bar item, which are always on.
func (e extensionsEditor) List() []api.ExtensionRecord {
	ctl := e.controller()
	if ctl == nil {
		return nil
	}
	running := e.menuBars()
	menuBars := map[string]bool{}
	for _, extension := range running {
		menuBars[extension.ID] = true
	}
	var records []api.ExtensionRecord
	for _, l := range ctl.List() {
		hasMenuBar := menuBars[l.ID]
		delete(menuBars, l.ID)
		records = append(records, api.ExtensionRecord{ID: l.ID, Title: extensionTitle(l), Enabled: l.Enabled,
			HasMenuBar: hasMenuBar, MenuBarShown: hasMenuBar && e.visibility.shown(l.ID), Capabilities: capabilityNames(l.Capabilities)})
	}
	for _, extension := range running {
		if menuBars[extension.ID] {
			records = append(records, api.ExtensionRecord{ID: extension.ID, Title: extension.Title, Enabled: true, BuiltIn: true,
				HasMenuBar: true, MenuBarShown: e.visibility.shown(extension.ID)})
		}
	}
	return records
}

// Disable turns a folder extension off, as the row's Disable does.
func (e extensionsEditor) Disable(id string) error {
	ctl, l, err := e.folderExtension(id)
	if err != nil {
		return err
	}
	if !l.Enabled {
		return extensionRefusal("%s is already off", extensionTitle(l))
	}
	if err := ctl.Disable(id); err != nil {
		return extensionRefusal("%s", err.Error())
	}
	return nil
}

// OpenReview shows the capability review of a disabled extension in the palette and returns at once. The review's own
// Enable is the only way on.
func (e extensionsEditor) OpenReview(id string) error {
	ctl, l, err := e.folderExtension(id)
	if err != nil {
		return err
	}
	if l.Enabled {
		return extensionRefusal("%s is already on", extensionTitle(l))
	}
	if !extensionCanEnable(l) {
		return extensionRefusal("%s does not load, so it cannot be enabled", extensionTitle(l))
	}
	e.onUI(func() { e.openReview(ctl, l) })
	return nil
}

// SetMenuBarItem shows or hides the menu bar item of a running extension, as the row's Show or Hide in Menu Bar does.
func (e extensionsEditor) SetMenuBarItem(id string, shown bool) error {
	for _, extension := range e.menuBars() {
		if extension.ID == id {
			e.visibility.setShown(id, shown)
			e.onUI(e.menuBarChanged)
			return nil
		}
	}
	return extensionRefusal("no running extension with a menu bar item has the id %q", id)
}

// folderExtension finds an installed folder extension. A built-in or an unknown id is refused.
func (e extensionsEditor) folderExtension(id string) (extensionController, manager.Listing, error) {
	ctl := e.controller()
	if ctl == nil {
		return nil, manager.Listing{}, extensionRefusal("extensions are still loading")
	}
	for _, l := range ctl.List() {
		if l.ID == id {
			return ctl, l, nil
		}
	}
	if e.builtIn(id) {
		return nil, manager.Listing{}, extensionRefusal("%s is built in and always on", id)
	}
	return nil, manager.Listing{}, extensionRefusal("no installed extension has the id %q", id)
}

func capabilityNames(capabilities []api.Capability) []string {
	names := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		names = append(names, string(capability))
	}
	return names
}

func extensionRefusal(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrInvalidInput)...)
}

// openExtensionReview shows the review of an extension above whatever the palette shows, such as the chat that asked
// for it, and gives the palette the keyboard. Enable and Cancel return to that page.
func (p *App) openExtensionReview(ctl extensionController, l manager.Listing) {
	if p.Palette == nil {
		return
	}
	p.Open(p.extensionReviewView(ctl, l))
	p.focusPanel()
}

func (p *App) extensionIsBuiltIn(id string) bool {
	m := p.extensionManager()
	return m != nil && m.IsBuiltIn(id)
}

func (p *App) runningMenuBarExtensions() []manager.MenuBarExtension {
	if m := p.extensionManager(); m != nil {
		return m.MenuBarExtensions()
	}
	return nil
}
