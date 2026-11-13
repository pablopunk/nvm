package manager

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtin"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// ErrBuiltIn is returned when a caller tries to turn a built-in extension off, remove it or read its folder. Built-ins
// are part of the app: they are always on and have no folder.
var ErrBuiltIn = errors.New("built-in extensions are always on")

// runner is what the manager asks of an extension's runtime: a child process for a folder extension, or the host
// process for a built-in.
type runner interface {
	Provides(method string) bool
	Status() supervisor.Status
	Call(ctx context.Context, method, query string, invocation *api.Invocation) (protocol.Response, error)
	Start()
	Stop()
	Reset()
	WaitFor(ctx context.Context, pred func(supervisor.Status) bool) error
}

// builtinKeyPrefix keeps a built-in's entry key apart from folder entries, which are keyed by folder name.
const builtinKeyPrefix = "builtin:"

// registerBuiltIns adds one entry per built-in. Each runs in the host with its own services, namespaced by its id.
func (m *Manager) registerBuiltIns(exts []api.Extension) {
	for _, ext := range exts {
		manifest := ext.Manifest()
		if err := manifest.ValidateBuiltIn(); err != nil {
			logBuiltInProblem(m.ports.Logs, manifest.ID, err)
			continue
		}
		host := m.builtInServices(manifest)
		serve := func(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
			return host.Serve(ctx, call)
		}
		m.entries[builtinKeyPrefix+manifest.ID] = &entry{
			bundle:  bundle.Bundle{Dir: "", Manifest: manifest},
			runtime: builtin.New(ext, m.ports.Clock, serve),
			host:    host,
			builtin: true,
		}
	}
}

// builtInServices returns the services of a built-in, with its settings in the host's store when the app names one.
func (m *Manager) builtInServices(manifest api.Manifest) *services.Handler {
	if m.builtInSettings != nil {
		if settings := m.builtInSettings(manifest.ID); settings != nil {
			return services.NewBuiltIn(m.serviceDeps, manifest, settings)
		}
	}
	return services.New(m.serviceDeps, manifest)
}

// isRunnable reports whether an entry's runtime may answer calls: built-ins always, folder extensions when they are
// enabled and work on this OS. Every listing of commands, rows, shortcuts and menu bars goes through it, so an extension
// that cannot run here contributes nothing to any of them.
func (m *Manager) isRunnable(e *entry) bool {
	return e.builtin || (m.isEnabled(e.bundle.Manifest) && m.supportedHere(e))
}

// IsBuiltIn reports whether id names a built-in extension.
func (m *Manager) IsBuiltIn(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[builtinKeyPrefix+id]
	return ok && e.builtin
}

// logBuiltInProblem records a built-in that is not registered. The logger may be absent in tests.
func logBuiltInProblem(logs *logx.Logger, id string, err error) {
	if logs != nil {
		logs.Error("built-in extension is not valid", logx.F{"id": id, "error": err.Error()})
	}
}
