package manager

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// CommandResult is one running extension's durable commands. Failure is set as for SearchResult.
type CommandResult struct {
	ExtensionID string
	Title       string
	Commands    []api.Command
	Failure     *api.Failure
}

// Commands asks every running extension for its durable commands and waits for each, within the Commands budget. The
// palette lists CachedCommands instead, so it never waits on a child.
func (m *Manager) Commands(ctx context.Context) []CommandResult {
	return m.commandsWithin(ctx, m.config.BudgetFor(protocol.MethodCommands))
}

func (m *Manager) commandsWithin(ctx context.Context, budget supervisor.Budget) []CommandResult {
	targets, _ := m.targetsFor(protocol.MethodCommands)
	results := make([]CommandResult, 0, len(targets))
	for _, target := range targets {
		if known, ok := m.commands.lookup(target.id); ok && isDormant(target) {
			results = append(results, known)
			continue
		}
		result := m.commandsOf(ctx, target, budget)
		if generation, ok := m.commands.begin(target.id, m.config.Clock.Now()); ok {
			m.rememberCommands(target, generation, result)
		}
		results = append(results, result)
	}
	return results
}

func (m *Manager) commandsOf(ctx context.Context, target runtimeTarget, budget supervisor.Budget) CommandResult {
	result := CommandResult{ExtensionID: target.id, Title: target.title}
	callCtx, cancel := m.callContext(protocol.WithSnapshotOrigin(ctx, target.identity), target.id, budget)
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
		applyCommandTitles(result.Commands, target.commandTitles)
	}
	return result
}

// applyCommandTitles gives each command the title the manifest renames it to, when it has one.
func applyCommandTitles(commands []api.Command, titles map[string]string) {
	for i := range commands {
		if title, ok := titles[commands[i].ID]; ok {
			commands[i].Title = title
		}
	}
}

// CommandMatches reports whether a command answers the query: its title, one of its aliases or one of its keywords
// matches the query as a phrase, a substring or a word. The root search lists a command for every query it matches.
func CommandMatches(command api.Command, query string) bool {
	texts := slices.Concat([]string{command.Title}, command.Aliases, command.Keywords)
	for _, text := range texts {
		if search.Score(text, query) >= search.PhraseScoreFloor {
			return true
		}
	}
	return false
}
