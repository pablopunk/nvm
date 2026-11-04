package windows

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/commands"
)

// ShortcutActions lists the window commands a keyboard shortcut can be bound to: the layouts and the focus moves, by
// their stable IDs. The views and the forms open the palette, so a shortcut does not take them.
func ShortcutActions() []shortcuts.Action {
	catalog := commands.All()
	actions := make([]shortcuts.Action, 0, len(catalog))
	for _, command := range catalog {
		actions = append(actions, shortcuts.Action{ID: command.ID, Title: command.Title})
	}
	return actions
}

// RunShortcut runs the window command a shortcut names, if actionID is one, and reports whether it did. The command acts
// on the app in front when the key was pressed. It never takes focus: the engine checks that the target is still in
// front, and the palette stays closed.
func RunShortcut(d Deps, actionID string) bool {
	command, ok := commands.ByID(actionID)
	if !ok {
		return false
	}
	d.runInBackground("shortcut", command.Title, func(ctx context.Context) string {
		inv := windowmgmt.Invocation{Source: windowmgmt.SourceShortcut}
		if target, found := d.Engine.FrontmostTarget(ctx); found {
			inv.Target = &target
		}
		return d.runner().Execute(ctx, command, inv)
	})
	return true
}
