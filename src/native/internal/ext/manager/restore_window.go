package manager

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// RestoreWindow asks extension id for the fresh view of the persistent window restoreKey after a relaunch. It returns
// nil when the extension no longer restores the key. It is bounded by the RestoreWindow budget.
func (m *Manager) RestoreWindow(ctx context.Context, id, restoreKey string) (*api.WindowRestore, error) {
	var restore *api.WindowRestore
	failure, err := m.callProvider(ctx, id, protocol.MethodRestoreWindow, &api.Invocation{Ref: api.ActionRef{ID: restoreKey}}, &restore)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return nil, &supervisor.CallError{Failure: *failure}
	}
	return restore, nil
}
