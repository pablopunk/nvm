package app

import (
	"sync"

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
