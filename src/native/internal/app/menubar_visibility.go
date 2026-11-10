package app

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// menuBarVisibilityFile is the store document of the extensions whose menu bar items are hidden. Items show by default.
const menuBarVisibilityFile = "menubar-visibility.json"

// menuBarVisibilityDocument lists the hidden extensions by id.
type menuBarVisibilityDocument struct {
	Hidden map[string]bool `json:"hidden"`
}

// menuBarVisibility is the per-extension choice to show its menu bar items, as 1.x keeps it. The item source reads it off
// the UI thread, so it has a lock.
type menuBarVisibility struct {
	mu     sync.Mutex
	doc    *store.Collection[menuBarVisibilityDocument]
	hidden map[string]bool
}

// restoreMenuBarVisibility opens the stored choices. A store that is not persisting keeps them for the run.
func (p *App) restoreMenuBarVisibility(st *store.Store) {
	if st == nil {
		return
	}
	p.visibility.doc = store.Doc(st, menuBarVisibilityFile, 1, func() menuBarVisibilityDocument { return menuBarVisibilityDocument{} })
	stored, err := p.visibility.doc.Load()
	if err != nil {
		return
	}
	p.visibility.mu.Lock()
	defer p.visibility.mu.Unlock()
	p.visibility.hidden = map[string]bool{}
	for id, hidden := range stored.Hidden {
		p.visibility.hidden[id] = hidden
	}
}

// shown reports whether the menu bar items of extension id show.
func (v *menuBarVisibility) shown(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return !v.hidden[id]
}

// setShown records the choice for extension id and stores it.
func (v *menuBarVisibility) setShown(id string, shown bool) {
	v.mu.Lock()
	if v.hidden == nil {
		v.hidden = map[string]bool{}
	}
	v.hidden[id] = !shown
	v.mu.Unlock()
	if v.doc != nil {
		_ = v.doc.Update(func(doc *menuBarVisibilityDocument) {
			if doc.Hidden == nil {
				doc.Hidden = map[string]bool{}
			}
			doc.Hidden[id] = !shown
		})
	}
}

// declaresMenuBar reports whether the running extension id declares menu bar items, which is when the row offers the choice.
func (p *App) declaresMenuBar(id string) bool {
	m := p.extensionManager()
	if m == nil {
		return false
	}
	for _, ext := range m.MenuBarExtensions() {
		if ext.ID == id {
			return true
		}
	}
	return false
}

// menuBarVisibilityAction is the row action that shows or hides the menu bar items of extension id, as 1.x's setting does.
// The items follow at once.
func (p *App) menuBarVisibilityAction(id string) search.Action {
	if p.visibility.shown(id) {
		return search.Action{Title: "Hide in Menu Bar", Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.setMenuBarShown(id, false, h)
		}}
	}
	return search.Action{Title: "Show in Menu Bar", Dismiss: search.DismissNever, Run: func(h search.Host) {
		p.setMenuBarShown(id, true, h)
	}}
}

func (p *App) setMenuBarShown(id string, shown bool, h search.Host) {
	p.visibility.setShown(id, shown)
	p.syncMenuBarItems()
	h.Refresh()
}

// menuBarToggleIDPrefix starts the command id of a menu bar toggle, the id 1.x gives the same row.
const menuBarToggleIDPrefix = "extension-menubar:"

// menuBarToggleCommands are the root rows that show or hide each menu bar extension. The host owns them, as in 1.x: an
// extension that adds a menu bar item gets its toggle without declaring anything.
func (p *App) menuBarToggleCommands() []command {
	m := p.extensionManager()
	if m == nil {
		return nil
	}
	var rows []command
	for _, extension := range m.MenuBarExtensions() {
		rows = append(rows, p.menuBarToggleCommand(extension))
	}
	return rows
}

// menuBarToggleCommand is the row of one extension. Its title stays the same whichever way it goes, so a search finds it
// the same way, and its subtitle says which way it is.
func (p *App) menuBarToggleCommand(extension manager.MenuBarExtension) command {
	id, title := extension.ID, extension.Title
	if title == "" {
		title = id
	}
	row := command{
		ID: menuBarToggleIDPrefix + id, Title: menuBarToggleTitle(title), Keywords: "menu bar menubar tray status bar show hide", Icon: "panel-top",
		PhraseMatch: true, Dismiss: search.DismissNever,
		Run: func(h search.Host) { p.setMenuBarShown(id, !p.visibility.shown(id), h) },
		extraActions: []search.Action{{Title: "Tweak with AI", Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(p.startedChatView("builder", "", id))
		}}},
	}
	if p.visibility.shown(id) {
		row.Subtitle = "Shown in the menu bar"
	} else {
		row.Subtitle = "Hidden from the menu bar"
	}
	return row
}

func menuBarToggleTitle(extensionTitle string) string {
	return "Toggle " + extensionTitle + " in Menu Bar"
}

// menuBarTogglePreviewItems is the toggle row of extension id in its preview, for an extension with menu bar items. The
// preview is not redrawn when it runs, so its subtitle does not name the current state.
func (p *App) menuBarTogglePreviewItems(id string) []api.Item {
	m := p.extensionManager()
	if m == nil {
		return nil
	}
	for _, extension := range m.MenuBarExtensions() {
		if extension.ID == id {
			return []api.Item{menuBarTogglePreviewItem(extension)}
		}
	}
	return nil
}

func menuBarTogglePreviewItem(extension manager.MenuBarExtension) api.Item {
	title := extension.Title
	if title == "" {
		title = extension.ID
	}
	return api.Item{
		ID: menuBarToggleIDPrefix + extension.ID, Title: menuBarToggleTitle(title), Subtitle: "Shows or hides it in the menu bar", Icon: "panel-top",
		Primary: &api.Action{Kind: api.ActionHost, Op: api.HostToggleMenuBar, Title: "Toggle"},
	}
}

// hostOperation runs the ActionHost op of a row. The toggle acts only on the extension whose row it is.
func (p *App) hostOperation(h search.Host, extID, op string) {
	if op == api.HostToggleMenuBar {
		if p.declaresMenuBar(extID) {
			p.setMenuBarShown(extID, !p.visibility.shown(extID), h)
		}
		return
	}
	p.settingsHostOperation(h, extID, op)
}
