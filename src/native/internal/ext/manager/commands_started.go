package manager

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// CommandsStarted lists the commands of every enabled extension for a caller that must act on the answer now, such as
// the command line on a cold start. It never relies on the palette having asked first: commands already known (saved
// from an earlier run, or learned) are used as they are, and an extension with none listed yet is started and waited
// for within ctx, then asked. Extensions are asked side by side, so one slow child delays only itself. A failed
// extension is reported with its Failure, as Commands does.
func (m *Manager) CommandsStarted(ctx context.Context) []CommandResult {
	targets := m.runnableTargets()
	results := make([]CommandResult, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = m.commandsStarted(ctx, target)
		}()
	}
	wg.Wait()
	return results
}

func (m *Manager) commandsStarted(ctx context.Context, target runtimeTarget) CommandResult {
	if known, ok := m.commands.lookup(target.id); ok {
		return known
	}
	target.runtime.Start()
	if err := target.runtime.WaitFor(ctx, canAnswerCommands); err != nil {
		return CommandResult{ExtensionID: target.id, Title: target.title, Failure: failureOf(err)}
	}
	result := m.commandsOf(ctx, target, m.config.BudgetFor(protocol.MethodCommands))
	if generation, ok := m.commands.begin(target.id, m.config.Clock.Now()); ok {
		m.rememberCommands(target, generation, result)
	}
	return result
}

// canAnswerCommands holds once an extension is past starting: it runs, waits dormant, or has failed and says why.
func canAnswerCommands(status supervisor.Status) bool {
	return status.State != supervisor.StateIdle && status.State != supervisor.StateStarting
}

// runnableTargets lists every enabled extension that has a runtime, whatever its state.
func (m *Manager) runnableTargets() []runtimeTarget {
	m.mu.Lock()
	defer m.mu.Unlock()
	var targets []runtimeTarget
	for _, key := range m.sortedKeysLocked() {
		if e := m.entries[key]; e.runtime != nil && m.isRunnable(e) {
			targets = append(targets, runtimeTarget{id: e.bundle.Manifest.ID, title: e.bundle.Manifest.Title, commandTitles: e.bundle.Manifest.CommandTitles, runtime: e.runtime})
		}
	}
	return targets
}
