package app

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/authoring"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Titles of the extension authoring actions, as 1.x titles them (App.tsx optionRows).
const (
	duplicateTitle    = "Duplicate"
	renameTitle       = "Rename"
	submitPRTitle     = "Submit as PR"
	tweakWithAITitle  = "Tweak with AI"
	removeActionTitle = "Remove action"
)

// extensionAuthoringActions are the actions 1.x adds to the menu of an extension row, after its shortcut and alias, in
// 1.x order: Duplicate and Submit as PR for an extension the AI builder made, Rename and Tweak with AI for any
// extension, and Remove action for an extension the builder made. Override with AI and Restore original are not listed:
// 1.x shows them only for a row with a default action, which no extension row has.
func (p *App) extensionAuthoringActions(extID, title string) []search.Action {
	owned := p.ownedByBuilder(extID)
	var out []search.Action
	if owned {
		out = append(out, p.duplicateAction(extID))
	}
	out = append(out, p.renameAction(extID))
	if owned {
		if action, ok := p.submitAction(extID); ok {
			out = append(out, action)
		}
	}
	out = append(out, search.Action{Title: tweakWithAITitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(p.builderEditView(extID))
	}})
	if owned {
		out = append(out, search.Action{Title: removeActionTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(p.removeActionConfirmation(extID, title))
		}})
	}
	return out
}

// duplicateAction copies the extension under a new id, installs the copy enabled, and opens the builder chat that tweaks it.
func (p *App) duplicateAction(extID string) search.Action {
	return search.Action{Title: duplicateTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		id, err := p.duplicateExtension(extID)
		if err != nil {
			h.Toast("Could not duplicate: " + err.Error())
			return
		}
		h.Push(p.builderEditView(id))
	}}
}

func (p *App) duplicateExtension(extID string) (string, error) {
	ctl := p.currentExtensionController()
	dir, ok := p.extensionFolder(extID)
	if ctl == nil || !ok {
		return "", errors.New("the extension is not installed")
	}
	id, err := authoring.Duplicate(p.ports.FS, filepath.Dir(dir), dir, time.Now())
	if err != nil {
		return "", err
	}
	if err := ctl.Refresh(); err != nil {
		return "", err
	}
	return id, ctl.Enable(id)
}

// renameAction asks for the extension's new name in a form, as 1.x's Rename prompt does.
func (p *App) renameAction(extID string) search.Action {
	return search.Action{Title: renameTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(p.renameForm(extID))
	}}
}

func (p *App) renameForm(extID string) *palette.View {
	title := p.extensionTitle(extID)
	return p.formView("Rename \""+title+"\"", []*FormField{{Label: "Name", Value: title, Placeholder: "Enter a new name"}}, func(vals []string) string {
		dir, ok := p.extensionFolder(extID)
		if !ok {
			return "The extension is not installed"
		}
		if err := authoring.Rename(p.ports.FS, dir, vals[0]); err != nil {
			return err.Error()
		}
		if ctl := p.currentExtensionController(); ctl != nil {
			_ = ctl.Refresh()
		}
		return ""
	})
}

// extensionFolder returns the folder of an installed extension and whether it is installed.
func (p *App) extensionFolder(extID string) (string, bool) {
	if l, ok := p.installedListing(extID); ok {
		return l.Dir, true
	}
	return "", false
}

func (p *App) extensionTitle(extID string) string {
	if l, ok := p.installedListing(extID); ok {
		return l.Title
	}
	return extID
}

func (p *App) installedListing(extID string) (manager.Listing, bool) {
	ctl := p.currentExtensionController()
	if ctl == nil {
		return manager.Listing{}, false
	}
	for _, l := range ctl.List() {
		if l.ID == extID {
			return l, true
		}
	}
	return manager.Listing{}, false
}

// removeActionConfirmation asks before the extension is removed from the extensions folder, with 1.x's wording.
func (p *App) removeActionConfirmation(extID, title string) *palette.View {
	return palette.Confirm(removeActionTitle, "Delete “"+title+"” from Nevermind", func() {
		if ctl := p.currentExtensionController(); ctl != nil {
			_ = ctl.Uninstall(extID)
		}
	})
}

// withAuthoringActions appends the authoring actions to a row of an extension the user installed. Built-in extensions
// have no source to author, so their rows are left as they are.
func (p *App) withAuthoringActions(it search.Item, extID string) search.Item {
	if p.extensionManager() == nil || p.extensionManager().IsBuiltIn(extID) {
		return it
	}
	it.Actions = append(it.Actions, p.extensionAuthoringActions(extID, it.Title)...)
	return it
}
