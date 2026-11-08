package manager

import (
	"context"
	"encoding/json"
	"sync/atomic"

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

// RestoreWindowBound returns a view and authority captured with the runtime that produced it.
func (m *Manager) RestoreWindowBound(ctx context.Context, id, restoreKey string) (*api.WindowRestore, *WindowAuthority, error) {
	m.mu.Lock()
	e, err := m.findLocked(id)
	if err != nil || e.runtime == nil || !m.isRunnable(e) {
		m.mu.Unlock()
		return nil, nil, ErrChangedInstallation
	}
	var identity any
	if !e.builtin {
		if e.snapshotIdentity == nil {
			e.snapshotIdentity = new(byte)
		}
		identity = e.snapshotIdentity
	}
	if e.revoked == nil {
		e.revoked = &atomic.Bool{}
	}
	runtime := e.runtime
	authority := &WindowAuthority{identity: identity, revoked: e.revoked}
	m.mu.Unlock()
	budget := m.config.BudgetFor(protocol.MethodRestoreWindow)
	callCtx, cancel := m.callContext(protocol.WithSnapshotOrigin(ctx, identity), id, budget)
	defer cancel()
	response, err := runtime.Call(callCtx, protocol.MethodRestoreWindow, "", &api.Invocation{Ref: api.ActionRef{ID: restoreKey}})
	if !authority.Valid() {
		return nil, authority, ErrChangedInstallation
	}
	if err != nil {
		return nil, authority, err
	}
	if response.Failure != nil {
		return nil, authority, &supervisor.CallError{Failure: *response.Failure}
	}
	var restore *api.WindowRestore
	if err := json.Unmarshal(response.Data, &restore); err != nil {
		return nil, authority, err
	}
	return restore, authority, nil
}
