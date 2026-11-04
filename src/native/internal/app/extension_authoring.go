package app

import (
	"os/exec"

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

// Disabled reasons for the authoring actions 2.0 cannot run yet. They name what is missing, so the menu says why.
const (
	duplicateUnavailable = "Duplicate is not in 2.0 yet"
	renameUnavailable    = "Rename is not in 2.0 yet"
	submitUnavailable    = "Submit as PR is not in 2.0 yet"
)

// extensionAuthoringActions are the actions 1.x adds to the menu of an extension row, after its shortcut and alias, in
// 1.x order: Duplicate and Submit as PR for an extension the AI builder made, Rename and Tweak with AI for any
// extension, and Remove action for an extension the builder made. Override with AI and Restore original are not listed:
// 1.x shows them only for a row with a default action, which no extension row has.
func (p *App) extensionAuthoringActions(extID, title string) []search.Action {
	owned := p.ownedByBuilder(extID)
	var out []search.Action
	if owned {
		out = append(out, search.Action{Title: duplicateTitle, Disabled: duplicateUnavailable})
	}
	out = append(out, search.Action{Title: renameTitle, Disabled: renameUnavailable})
	if owned && ghInstalled() {
		out = append(out, search.Action{Title: submitPRTitle, Disabled: submitUnavailable})
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

// removeActionConfirmation asks before the extension is removed from the extensions folder, with 1.x's wording.
func (p *App) removeActionConfirmation(extID, title string) *palette.View {
	return palette.Confirm(removeActionTitle, "Delete “"+title+"” from Nevermind", func() {
		if ctl := p.currentExtensionController(); ctl != nil {
			_ = ctl.Uninstall(extID)
		}
	})
}

// ghInstalled reports whether the GitHub CLI is on the PATH, which decides whether 1.x offers Submit as PR at all.
func ghInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
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
