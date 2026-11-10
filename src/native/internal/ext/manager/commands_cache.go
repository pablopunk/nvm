package manager

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"reflect"
	"sync"
	"time"
)

const commandRetryInterval = 2 * time.Second

// InvalidateCommands revalidates an extension's commands after an explicit root invalidation while retaining its snapshot.
func (m *Manager) InvalidateCommands(id string) {
	m.commands.invalidate(id)
}

type commandCache struct {
	mu         sync.Mutex
	known      map[string]CommandResult
	inflight   map[string]bool
	retryAt    map[string]time.Time
	generation map[string]uint64
}

// CachedCommands returns known commands immediately and discovers unknown commands independently in the background.
// Built-ins use the same path as folder extensions; neither can block palette input or another extension's commands.
func (m *Manager) CachedCommands() []CommandResult {
	targets, _ := m.targetsFor(protocol.MethodCommands)
	out := make([]CommandResult, 0, len(targets))
	for _, target := range targets {
		if result, ok := m.commands.lookup(target.id); ok {
			out = append(out, result)
			if isDormant(target) {
				continue
			}
		}
		if generation, ok := m.commands.begin(target.id, m.config.Clock.Now()); ok {
			go m.learnCommands(target, generation)
		}
	}
	return out
}

func (m *Manager) learnCommands(target runtimeTarget, generation uint64) {
	result := m.commandsOf(context.Background(), target, m.config.BudgetFor(protocol.MethodCommands))
	if m.rememberCommands(target, generation, result) && m.commandsChange != nil {
		m.commandsChange()
	}
}

func (m *Manager) rememberCommands(target runtimeTarget, generation uint64, result CommandResult) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false
	}
	entry, err := m.findLocked(target.id)
	if err != nil || entry.runtime != target.runtime || !m.isRunnable(entry) {
		return false
	}
	changed := m.commands.finish(target.id, generation, result, m.config.Clock.Now())
	if result.Failure == nil {
		m.snapshots.put(target.id, entry.bundle.Hash, result.Commands)
	}
	return changed
}

// seed records commands saved by an earlier run, so a dormant extension lists them without starting.
func (c *commandCache) seed(id string, result CommandResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	c.known[id] = result
}

func (c *commandCache) lookup(id string) (CommandResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.known[id]
	return result, ok
}

func (c *commandCache) begin(id string, now time.Time) (uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	if c.inflight[id] || now.Before(c.retryAt[id]) {
		return 0, false
	}
	c.inflight[id] = true
	return c.generation[id], true
}

func (c *commandCache) finish(id string, generation uint64, result CommandResult, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	if generation != c.generation[id] {
		return false
	}
	delete(c.inflight, id)
	if result.Failure != nil {
		c.retryAt[id] = now.Add(commandRetryInterval)
		return false
	}
	previous, known := c.known[id]
	c.known[id] = result
	c.retryAt[id] = now.Add(commandRetryInterval)
	return !known || !reflect.DeepEqual(previous, result)
}

func (c *commandCache) ensureLocked() {
	if c.known == nil {
		c.known, c.inflight, c.retryAt, c.generation = map[string]CommandResult{}, map[string]bool{}, map[string]time.Time{}, map[string]uint64{}
	}
}

func (c *commandCache) forget(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	c.generation[id]++
	delete(c.known, id)
	delete(c.inflight, id)
	delete(c.retryAt, id)
}

func (c *commandCache) invalidate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLocked()
	c.generation[id]++
	delete(c.inflight, id)
	delete(c.retryAt, id)
}
