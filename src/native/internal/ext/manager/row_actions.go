package manager

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// RowAction is an action a built-in adds to a row, with the id of the built-in that runs it.
type RowAction struct {
	Extension string
	Action    api.Action
}

// RowActions asks the built-in extensions for the actions they add to the menu of an installed extension's row, in the
// order of their ids. A built-in that fails, or does not implement RowActionProvider, adds none.
func (m *Manager) RowActions(ctx context.Context, row api.ExtensionRow) []RowAction {
	targets, _ := m.targetsFor(protocol.MethodRowActions)
	var out []RowAction
	for _, target := range targets {
		if !m.IsBuiltIn(target.id) {
			continue
		}
		var actions []api.Action
		invocation := &api.Invocation{Ref: api.ActionRef{ID: protocol.MethodRowActions, Args: row.Args()}}
		if failure, err := m.callProvider(ctx, target.id, protocol.MethodRowActions, invocation, &actions); failure == nil && err == nil {
			for _, action := range actions {
				out = append(out, RowAction{Extension: target.id, Action: action})
			}
		}
	}
	return out
}
