package manager

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

func TestResolveCommandOriginUsesResolvedBuiltinAndKnownCommand(t *testing.T) {
	builtin := &entry{builtin: true, bundle: bundleForOrigin("builtin.sample")}
	shadow := &entry{bundle: bundleForOrigin("folder.extension")}
	m := &Manager{entries: map[string]*entry{"builtin:builtin.sample": builtin, "builtin.sample": shadow}}
	m.commands.known = map[string]CommandResult{"builtin.sample": {ExtensionID: "builtin.sample", Commands: []api.Command{{ID: "run"}}}}
	if got := m.ResolveCommandOrigin("builtin.sample", "run"); got.ExtensionID() != "" {
		t.Fatalf("shadow resolved as builtin: %#v", got)
	}
	delete(m.entries, "builtin.sample")
	got := m.ResolveCommandOrigin("builtin.sample", "run")
	if got.ExtensionID() != "builtin.sample" || got.CommandID() != "run" {
		t.Fatalf("unexpected origin: %#v", got)
	}
	if got := m.ResolveCommandOrigin("builtin.sample", "missing"); got.ExtensionID() != "builtin.sample" || got.CommandID() != "" {
		t.Fatalf("unknown command retained: %#v", got)
	}
}

func TestResolveCommandOriginStripsExternalAndHandlesUnknown(t *testing.T) {
	m := &Manager{entries: map[string]*entry{"folder-alias": {bundle: bundleForOrigin("folder.extension")}}}
	if got := m.ResolveCommandOrigin("folder-alias", "run"); got != (CommandOrigin{}) {
		t.Fatalf("external IDs leaked: %#v", got)
	}
	if got := m.ResolveCommandOrigin("missing", "run"); got != (CommandOrigin{}) {
		t.Fatalf("unknown IDs leaked: %#v", got)
	}
	var nilManager *Manager
	if got := nilManager.ResolveCommandOrigin("anything", "run"); got != (CommandOrigin{}) {
		t.Fatalf("nil manager: %#v", got)
	}
}

func bundleForOrigin(id string) bundle.Bundle {
	return bundle.Bundle{Manifest: api.Manifest{Identity: api.Identity{ID: id}}}
}

func TestResolveCommandOriginOnlyRetainsValidatedKnownIDs(t *testing.T) {
	for _, id := range []string{"Builtin:Sample", strings.Repeat("a", 64), "bad id", "_bad", strings.Repeat("a", 65)} {
		m := &Manager{entries: map[string]*entry{"alias": {builtin: true, bundle: bundleForOrigin(id)}}}
		origin := m.ResolveCommandOrigin("alias", "private-command-not-in-cache")
		if validOriginID(id) {
			if origin.ExtensionID() != id || origin.CommandID() != "" {
				t.Fatalf("uncached origin = %#v", origin)
			}
		} else if origin != (CommandOrigin{}) {
			t.Fatalf("invalid ID retained: %#v", origin)
		}
	}
}
