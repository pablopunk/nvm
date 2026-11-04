package manager

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// CommandResult is one running extension's durable commands. Failure is set as for SearchResult.
type CommandResult struct {
	ExtensionID string
	Title       string
	Commands    []api.Command
	Failure     *api.Failure
}

// Commands asks every running extension for its durable commands, each within the Commands budget. The root
// lists them when the query is empty.
func (m *Manager) Commands(ctx context.Context) []CommandResult {
	return m.commandsWithin(ctx, m.config.BudgetFor(protocol.MethodCommands))
}

// SearchCommands asks every running extension for its durable commands within the Search budget, so a typed
// query that also matches commands is never held up by a slow extension for longer than a search is.
func (m *Manager) SearchCommands(ctx context.Context) []CommandResult {
	return m.commandsWithin(ctx, m.config.BudgetFor(protocol.MethodSearch))
}

func (m *Manager) commandsWithin(ctx context.Context, budget supervisor.Budget) []CommandResult {
	targets, _ := m.searchTargets()
	results := make([]CommandResult, 0, len(targets))
	for _, target := range targets {
		results = append(results, m.commandsOf(ctx, target, budget))
	}
	return results
}

func (m *Manager) commandsOf(ctx context.Context, target runtimeTarget, budget supervisor.Budget) CommandResult {
	result := CommandResult{ExtensionID: target.id, Title: target.title}
	callCtx, cancel := context.WithTimeout(ctx, budget.Hard+budgetSlack)
	defer cancel()
	response, err := target.runtime.Call(callCtx, protocol.MethodCommands, "", nil)
	switch {
	case err != nil:
		result.Failure = failureOf(err)
	case response.Failure != nil:
		result.Failure = response.Failure
	default:
		if err := json.Unmarshal(response.Data, &result.Commands); err != nil {
			result.Failure = &api.Failure{Code: api.CodeInternal, Message: "commands are not valid"}
		}
	}
	return result
}
