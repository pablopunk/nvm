package manager

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Load asks extension id for the items of one of its data loaders. Like Run it is bounded by the host's budget, and a
// failure comes back as a wire failure, never as a panic. The palette calls it off the UI thread.
func (m *Manager) Load(ctx context.Context, id, loader string) ([]api.Item, *api.Failure, error) {
	runtime, err := m.runtimeFor(id)
	if err != nil {
		return nil, nil, err
	}
	budget := m.config.BudgetFor(protocol.MethodRun)
	callCtx, cancel := context.WithTimeout(ctx, budget.Hard+budgetSlack)
	defer cancel()
	response, err := runtime.Call(callCtx, protocol.MethodLoad, "", &api.Invocation{Ref: api.ActionRef{ID: loader}})
	if err != nil {
		return nil, failureOf(err), nil
	}
	if response.Failure != nil {
		return nil, response.Failure, nil
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil {
		return nil, &api.Failure{Code: api.CodeInternal, Message: "the loader returned items that are not valid"}, nil
	}
	return items, nil, nil
}
