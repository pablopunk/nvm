package shortcuts

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// ExtensionAliasEditor changes the user's aliases for extensions and conversation programs. An alias belongs to one
// action, so a phrase never finds two rows by the user's choice; commands are named from the shortcut snapshot.
type ExtensionAliasEditor struct {
	table       ExtensionTable
	afterChange func()
}

// NewExtensionAliasEditor returns the alias editor over table's aliases. afterChange, when set, runs after a change was
// saved, so the app can drop root rows built from the old aliases.
func NewExtensionAliasEditor(table ExtensionTable, afterChange func()) ExtensionAliasEditor {
	return ExtensionAliasEditor{table: table, afterChange: afterChange}
}

// PlanAlias prepares adding alias to actionID, or removing alias when actionID is empty. It refuses what the change
// would refuse, so nobody is asked to approve a change that cannot happen.
func (e ExtensionAliasEditor) PlanAlias(actionID, alias string) (protocol.AliasChange, error) {
	cleaned, err := CleanAlias(alias)
	change := protocol.AliasChange{ActionID: actionID, Alias: cleaned}
	if err != nil {
		return change, err
	}
	if actionID == "" {
		return e.planRemoval(change)
	}
	entry, found := e.table.manager.Snapshot().Entry(actionID)
	if !found {
		return change, fmt.Errorf("no command has the action id %q; list the bindable ones first: %w", actionID, api.ErrInvalidInput)
	}
	change.Title = entry.Title
	if owner, taken := e.table.aliases.Owner(cleaned); taken {
		return change, e.aliasTaken(cleaned, owner)
	}
	return change, nil
}

func (e ExtensionAliasEditor) planRemoval(change protocol.AliasChange) (protocol.AliasChange, error) {
	owner, found := e.table.aliases.Owner(change.Alias)
	if !found {
		return change, fmt.Errorf("no command has the alias %q: %w", change.Alias, api.ErrInvalidInput)
	}
	change.ActionID, change.Title = owner, e.titleOf(owner)
	return change, nil
}

// AddAlias saves alias for actionID. The check that no action has it runs again as it is saved.
func (e ExtensionAliasEditor) AddAlias(actionID, alias string) error {
	change, err := e.PlanAlias(actionID, alias)
	if err != nil {
		return err
	}
	owner, err := e.table.aliases.Claim(actionID, change.Alias)
	if owner != "" {
		return e.aliasTaken(change.Alias, owner)
	}
	if err != nil {
		return err
	}
	e.changed()
	return nil
}

// RemoveAlias deletes alias from the action that has it.
func (e ExtensionAliasEditor) RemoveAlias(alias string) error {
	change, err := e.PlanAlias("", alias)
	if err != nil {
		return err
	}
	if err := e.table.aliases.Drop(change.Alias); err != nil {
		return err
	}
	e.changed()
	return nil
}

func (e ExtensionAliasEditor) aliasTaken(alias, owner string) error {
	return fmt.Errorf("“%s” is already an alias of %s: %w", alias, e.titleOf(owner), api.ErrInvalidInput)
}

// titleOf names an action by its title, or by its ID when the table no longer has it.
func (e ExtensionAliasEditor) titleOf(actionID string) string {
	if entry, found := e.table.manager.Snapshot().Entry(actionID); found && entry.Title != "" {
		return entry.Title
	}
	return actionID
}

func (e ExtensionAliasEditor) changed() {
	if e.afterChange != nil {
		e.afterChange()
	}
}
