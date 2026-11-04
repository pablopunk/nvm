package manager

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// MenuBarExtension is an enabled extension whose running child declares menu bar items. Revision is the hash of the
// version on disk: a reinstall changes it, so the host reads the declarations again.
type MenuBarExtension struct {
	ID       string
	Revision string
}

// MenuBarExtensions lists the enabled extensions whose running child declares menu bar items. An extension that is
// starting or stopped is left out until its next listing, so a menu bar never waits on a child.
func (m *Manager) MenuBarExtensions() []MenuBarExtension {
	m.mu.Lock()
	defer m.mu.Unlock()
	var listed []MenuBarExtension
	for _, key := range m.sortedKeysLocked() {
		e := m.entries[key]
		if e.runtime != nil && m.isRunnable(e) && e.runtime.Provides(protocol.MethodMenuBars) {
			listed = append(listed, MenuBarExtension{ID: e.bundle.Manifest.ID, Revision: e.bundle.Hash})
		}
	}
	return listed
}

// MenuBars asks extension id to render its status items. It is bounded by the MenuBars budget; a failure comes back as
// a wire failure, and the caller keeps what it showed before.
func (m *Manager) MenuBars(ctx context.Context, id string) ([]api.MenuBar, *api.Failure, error) {
	var bars []api.MenuBar
	failure, err := m.callProvider(ctx, id, protocol.MethodMenuBars, nil, &bars)
	return bars, failure, err
}

// MenuBarTriggers returns the triggers that re-render extension id's status items.
func (m *Manager) MenuBarTriggers(ctx context.Context, id string) ([]api.Trigger, *api.Failure, error) {
	var triggers []api.Trigger
	failure, err := m.callProvider(ctx, id, protocol.MethodMenuBarTriggers, nil, &triggers)
	return triggers, failure, err
}

// callProvider runs one provider method of extension id within its budget and decodes the result into into.
func (m *Manager) callProvider(ctx context.Context, id, method string, invocation *api.Invocation, into any) (*api.Failure, error) {
	runtime, err := m.runtimeFor(id)
	if err != nil {
		return nil, err
	}
	budget := m.config.BudgetFor(method)
	callCtx, cancel := context.WithTimeout(ctx, budget.Hard+budgetSlack)
	defer cancel()
	response, err := runtime.Call(callCtx, method, "", invocation)
	if err != nil {
		return failureOf(err), nil
	}
	if response.Failure != nil {
		return response.Failure, nil
	}
	if err := json.Unmarshal(response.Data, into); err != nil {
		return &api.Failure{Code: api.CodeInternal, Message: method + " returned data that is not valid"}, nil
	}
	return nil, nil
}
