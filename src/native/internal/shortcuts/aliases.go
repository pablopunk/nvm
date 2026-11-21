package shortcuts

import (
	"slices"
	"strings"
	"sync"
)

// AliasPersistence is where the aliases live: config.json in the app, memory when there is no config store.
type AliasPersistence interface {
	Load() (map[string][]string, error)
	Update(change func(map[string][]string)) error
}

// Aliases is the alias list of each action: the phrases that find the action in the root search. The list is read
// once and kept in memory, because the root reads it on every keystroke.
type Aliases struct {
	mu     sync.Mutex
	list   AliasPersistence
	cached map[string][]string
	loaded bool
}

// NewAliases returns the aliases over a persistence.
func NewAliases(list AliasPersistence) *Aliases {
	return &Aliases{list: list}
}

// All returns a copy of every action's aliases.
func (a *Aliases) All() map[string][]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneAliases(a.current())
}

// For returns the aliases of one action, in the order they were added.
func (a *Aliases) For(actionID string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.current()[actionID])
}

// Add saves alias for actionID. A blank alias and a repeat change nothing.
func (a *Aliases) Add(actionID, alias string) error {
	alias = strings.TrimSpace(alias)
	if actionID == "" || alias == "" {
		return nil
	}
	return a.update(func(all map[string][]string) {
		if !slices.Contains(all[actionID], alias) {
			all[actionID] = append(all[actionID], alias)
		}
	})
}

// Remove deletes alias from actionID. An action with no aliases left has no entry.
func (a *Aliases) Remove(actionID, alias string) error {
	return a.update(func(all map[string][]string) {
		remaining := slices.DeleteFunc(slices.Clone(all[actionID]), func(value string) bool { return value == alias })
		if len(remaining) == 0 {
			delete(all, actionID)
			return
		}
		all[actionID] = remaining
	})
}

// current loads the aliases on first use. The caller holds mu.
func (a *Aliases) current() map[string][]string {
	if !a.loaded {
		a.cached, _ = a.list.Load()
		if a.cached == nil {
			a.cached = map[string][]string{}
		}
		a.loaded = true
	}
	return a.cached
}

func (a *Aliases) update(change func(map[string][]string)) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.list.Update(change)
	a.loaded = false
	return err
}

// MemoryAliases keeps the aliases in memory. Changes last as long as the value does.
type MemoryAliases struct {
	mu  sync.Mutex
	all map[string][]string
}

// NewMemoryAliases returns an empty memory persistence.
func NewMemoryAliases() *MemoryAliases {
	return &MemoryAliases{all: map[string][]string{}}
}

// Load returns a copy of the aliases.
func (m *MemoryAliases) Load() (map[string][]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneAliases(m.all), nil
}

// Update applies change to the aliases.
func (m *MemoryAliases) Update(change func(map[string][]string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	change(m.all)
	return nil
}

func cloneAliases(all map[string][]string) map[string][]string {
	out := make(map[string][]string, len(all))
	for id, list := range all {
		out[id] = slices.Clone(list)
	}
	return out
}
