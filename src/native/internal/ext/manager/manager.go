// Package manager is the host's one view of the extensions on disk: it scans the extensions folder, keeps one
// supervised runtime per runnable extension, hot-reloads a folder whose content changed, and answers the
// palette's questions (search, run, list, source). Installs and removals go through package install, so a
// failed write never leaves a visible half-extension behind.
//
// Layer: ext. The package imports api, bundle, enablement, install, protocol, supervisor, platform and the
// standard library.
package manager

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/install"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Ports are the OS and time services the manager uses. The composition root supplies the real set; tests can
// substitute fakes.
type Ports struct {
	Files     platform.FS
	Clock     platform.Clock
	Worker    platform.Worker
	Processes platform.Processes
}

// Options tune the child processes of every extension.
type Options struct {
	// Executable is this binary, which serves the child role.
	Executable    string
	MemoryLimit   uint64
	ResidentLimit uint64
	Budgets       map[string]supervisor.Budget
}

// Manager owns the extensions folder root. It is safe for concurrent use.
type Manager struct {
	ports   Ports
	root    string
	enabled *enablement.Set
	config  supervisor.Config
	mu      sync.Mutex
	entries map[string]*entry
}

// entry is one extension folder: the version on disk and the runtime that runs it, when it can run.
type entry struct {
	bundle  bundle.Bundle
	runtime *supervisor.Runtime
}

// New opens the extensions folder root, removes leftovers of interrupted installs, and loads every extension.
// Enabled extensions start in the background; nothing here waits for them.
func New(ports Ports, root string, enabled *enablement.Set, opts Options) (*Manager, error) {
	if err := install.Sweep(ports.Files, root); err != nil {
		return nil, err
	}
	m := &Manager{
		ports:   ports,
		root:    root,
		enabled: enabled,
		config: supervisor.Config{
			Worker:        ports.Worker,
			Clock:         ports.Clock,
			Processes:     ports.Processes,
			Executable:    opts.Executable,
			MemoryLimit:   opts.MemoryLimit,
			ResidentLimit: opts.ResidentLimit,
			Budgets:       opts.Budgets,
		},
		entries: map[string]*entry{},
	}
	if err := m.Refresh(); err != nil {
		return nil, err
	}
	return m, nil
}

// Refresh rescans the folder. A new folder is loaded, a changed one is replaced (its old runtime stops and the
// new one starts when enabled), and a removed one is stopped and forgotten.
func (m *Manager) Refresh() error {
	bundles, err := bundle.Scan(m.ports.Files, m.root)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked(bundles)
	return nil
}

func (m *Manager) refreshLocked(bundles []bundle.Bundle) {
	seen := map[string]bool{}
	for _, loaded := range bundles {
		key := filepath.Base(loaded.Dir)
		seen[key] = true
		if current, ok := m.entries[key]; ok && current.bundle.Hash == loaded.Hash {
			continue
		}
		m.replaceLocked(key, loaded)
	}
	for key, current := range m.entries {
		if !seen[key] {
			stopEntry(current)
			delete(m.entries, key)
		}
	}
}

// replaceLocked swaps the entry for key with one holding loaded. The old runtime is stopped first, so two
// versions never run at once.
func (m *Manager) replaceLocked(key string, loaded bundle.Bundle) {
	if old, ok := m.entries[key]; ok {
		stopEntry(old)
	}
	next := &entry{bundle: loaded}
	if loaded.Runnable() {
		next.runtime = supervisor.New(m.config, loaded.Manifest.Identity, filepath.Join(loaded.Dir, bundle.SourceFile))
		if m.isEnabled(loaded.Manifest.ID) {
			next.runtime.Start()
		}
	}
	m.entries[key] = next
}

func stopEntry(e *entry) {
	if e.runtime != nil {
		e.runtime.Stop()
	}
}

// isEnabled reads the enabled state. A load error counts as disabled, so a damaged file never turns code on.
func (m *Manager) isEnabled(id string) bool {
	enabled, err := m.enabled.Enabled(id)
	return err == nil && enabled
}

// sortedKeysLocked returns the entry keys in order, for stable listings.
func (m *Manager) sortedKeysLocked() []string {
	keys := make([]string, 0, len(m.entries))
	for key := range m.entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Close stops every runtime. The manager must not be used afterwards.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		stopEntry(e)
	}
}
