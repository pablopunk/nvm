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

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ai/quota"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/install"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Ports are the OS and time services the manager uses. The composition root supplies the real set; tests can
// substitute fakes. Keychain, Network (the public-address port), Clipboard and Logs feed the context services;
// nil ones make the services that need them unavailable.
type Ports struct {
	Files     platform.FS
	Clock     platform.Clock
	Worker    platform.Worker
	Processes platform.Processes
	Keychain  platform.Keychain
	Network   platform.HTTP
	Clipboard platform.Clipboard
	Logs      *logx.Logger
	// The desktop services. Each one is optional: a nil port answers unavailable for the services that need it.
	Shell        platform.Shell
	Commands     platform.Commands
	Capabilities platform.Capabilities
	OS           string
	Windows      services.WindowEngine
	// Frontmost names the app in front for the apps service. Nil answers unavailable.
	Frontmost services.FrontmostSource
	// Notifier shows the notifications of the system service. Nil answers unavailable.
	Notifier  platform.Notifier
	Shortcuts services.ShortcutTable
	Account   services.AccountState
	OCR       *ocr.Service
	History   func() *clipboard.History
	Confirm   services.ConfirmFunc
	// SettingsPanes lists the operating system's settings panes for the system service.
	SettingsPanes func() []api.SettingsPane
	// Places lists the standard folders of the home folder for the system service.
	Places func() []api.Place
	// Indicators draws the passive indicators of extensions.
	Indicators services.IndicatorSurface
	// AI makes the model calls of extensions. Nil makes ctx.AI unavailable.
	AI *extai.Service
	// Selection reads, replaces and undoes the selected text of extensions. Nil makes ctx.desktop.selection unavailable.
	Selection services.SelectionEngine
	// Apps quits, force quits, hides and reports the running state of the indexed apps. Nil makes them unavailable.
	Apps services.AppLifecycle
	// Paste sends the paste and typing actions of extensions to the app in front. Nil makes them unavailable.
	Paste services.PasteEngine
	// Notify shows a failure of an extension's action to the user, such as a refused AppleScript. Nil logs only.
	Notify services.NotifyFunc
}

// Options tune the child processes of every extension.
type Options struct {
	// Executable is this binary, which serves the child role.
	Executable    string
	MemoryLimit   uint64
	ResidentLimit uint64
	Budgets       map[string]supervisor.Budget
	// DataRoot holds one data folder per extension, named after its id.
	DataRoot string
	// Home is the user's home folder, which "~" in shell arguments stands for.
	Home string
	// OnChange runs after any extension's runtime changes state, so a view showing it can redraw. It runs on its
	// own goroutine and must not block.
	OnChange func()
	// MenuBarRefresh receives an extension's RefreshMenuBar calls. It must not block.
	MenuBarRefresh func(extensionID string)
	// RootRefresh receives an extension's InvalidateRoot calls. It must not block.
	RootRefresh func(extensionID string)
	// AIQuota shares each extension's model-call count with the app's other callers of the same owner.
	AIQuota *quota.Registry
	// SearchRefresh receives an extension's RefreshSearch calls. It must not block.
	SearchRefresh func(extensionID, query string)
	// BuiltIns are the extensions compiled into the app. They run in the host and are always on.
	BuiltIns []api.Extension
	// BuiltInSettings returns the host's store for a built-in's settings, such as config.json, or nil when the built-in
	// keeps its settings in its own file.
	BuiltInSettings func(id string) services.SettingsHost
	// LegacyStatePath is the 1.x state file. A folder that names a 1.x source file takes its enabled state from it on
	// its first scan. Empty means no 1.x state is read.
	LegacyStatePath string
}

// Manager owns the extensions folder root. It is safe for concurrent use.
type Manager struct {
	ports       Ports
	root        string
	enabled     *enablement.Set
	config      supervisor.Config
	serviceDeps services.Deps
	// builtInSettings is Options.BuiltInSettings.
	builtInSettings func(id string) services.SettingsHost
	mu              sync.Mutex
	entries         map[string]*entry
	// legacyStatePath is the 1.x state file read by inheritLocked.
	legacyStatePath string
}

// entry is one extension folder: the version on disk, the runtime that runs it when it can run, and the host
// services that runtime calls.
type entry struct {
	bundle  bundle.Bundle
	runtime runner
	host    *services.Handler
	// builtin marks an entry compiled into the app: it has no folder, no review and no enabled state.
	builtin bool
}

// New opens the extensions folder root, removes leftovers of interrupted installs, and loads every extension.
// Enabled extensions start in the background; nothing here waits for them.
func New(ports Ports, root string, enabled *enablement.Set, opts Options) (*Manager, error) {
	if err := install.Sweep(ports.Files, root); err != nil {
		return nil, err
	}
	m := &Manager{
		ports:           ports,
		root:            root,
		enabled:         enabled,
		builtInSettings: opts.BuiltInSettings,
		config: supervisor.Config{
			Worker:        ports.Worker,
			Clock:         ports.Clock,
			Processes:     ports.Processes,
			Executable:    opts.Executable,
			MemoryLimit:   opts.MemoryLimit,
			ResidentLimit: opts.ResidentLimit,
			Budgets:       opts.Budgets,
			OnStateChange: opts.OnChange,
			Log:           ports.Logs,
		},
		serviceDeps: services.Deps{
			Files:          ports.Files,
			Clock:          ports.Clock,
			DataRoot:       opts.DataRoot,
			Keychain:       ports.Keychain,
			Network:        ports.Network,
			Clipboard:      ports.Clipboard,
			Logs:           ports.Logs,
			MenuBarRefresh: opts.MenuBarRefresh,
			RootRefresh:    opts.RootRefresh,
			AIQuota:        opts.AIQuota,
			SearchRefresh:  opts.SearchRefresh,

			Shell:        ports.Shell,
			Commands:     ports.Commands,
			Home:         opts.Home,
			Capabilities: ports.Capabilities,
			OS:           ports.OS,
			Windows:      ports.Windows,
			Frontmost:    ports.Frontmost,
			Notifier:     ports.Notifier,
			Shortcuts:    ports.Shortcuts,
			Account:      ports.Account,
			OCR:          ports.OCR,
			Indicators:   ports.Indicators,
			AI:           ports.AI,
			Selection:    ports.Selection,
			Paste:        ports.Paste,
			Apps:         ports.Apps,
			History:      ports.History,
			Confirm:      ports.Confirm,
			Notify:       ports.Notify,

			SettingsPanes: ports.SettingsPanes,
			Places:        ports.Places,
		},
		entries:         map[string]*entry{},
		legacyStatePath: opts.LegacyStatePath,
	}
	m.serviceDeps.RunScheduled = m.runScheduled
	m.registerBuiltIns(opts.BuiltIns)
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
	m.inheritLocked(bundles)
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
		if !seen[key] && !current.builtin {
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
		next.host = services.New(m.serviceDeps, loaded.Manifest)
		next.runtime = supervisor.New(m.config, loaded.Manifest.Identity, filepath.Join(loaded.Dir, bundle.SourceFile), next.host)
		if m.isEnabled(loaded.Manifest.ID) {
			next.runtime.Start()
		}
	}
	m.entries[key] = next
}

// stopEntry stops the entry's child and then closes its services, which flushes the extension's data.
func stopEntry(e *entry) {
	if e.runtime != nil {
		e.runtime.Stop()
	}
	if e.host != nil {
		_ = e.host.Close()
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
