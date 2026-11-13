package manager

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

type stateRunner struct{ state supervisor.State }

func (r stateRunner) Provides(string) bool                                        { return true }
func (r stateRunner) Status() supervisor.Status                                   { return supervisor.Status{State: r.state} }
func (r stateRunner) Start()                                                      {}
func (r stateRunner) Stop()                                                       {}
func (r stateRunner) Reset()                                                      {}
func (r stateRunner) WaitFor(context.Context, func(supervisor.Status) bool) error { return nil }
func (r stateRunner) Call(context.Context, string, string, *api.Invocation) (protocol.Response, error) {
	return protocol.Response{}, nil
}

// A known command must stay in the root while its extension's process is idle or restarting: what an extension
// declares does not depend on whether its process is up at this moment.
func TestKnownCommandsStayListedWhileTheirExtensionRestarts(t *testing.T) {
	for _, state := range []supervisor.State{supervisor.StateIdle, supervisor.StateStarting} {
		m := &Manager{entries: map[string]*entry{
			builtinKeyPrefix + "acme.walls": {builtin: true, runtime: stateRunner{state: state}, bundle: bundleWithID("acme.walls")},
		}}
		m.commands.seed("acme.walls", CommandResult{ExtensionID: "acme.walls", Commands: []api.Command{{ID: "online", Title: "Online Wallpapers"}}})
		got := m.CachedCommands()
		if len(got) != 1 || got[0].Commands[0].Title != "Online Wallpapers" {
			t.Fatalf("state %v: commands %v", state, got)
		}
	}
}

func bundleWithID(id string) bundle.Bundle { return bundle.Bundle{Manifest: api.Manifest{ID: id}} }
