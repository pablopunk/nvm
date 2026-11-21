package manager

import (
	"context"
	"strings"
	"testing"
)

func TestResolveViewActionOriginUsesRegistryEntry(t *testing.T) {
	builtin := &entry{builtin: true, bundle: bundleForOrigin("builtin.sample")}
	user := &entry{bundle: bundleForOrigin("builtin.lookalike")}
	m := &Manager{entries: map[string]*entry{"builtin:builtin.sample": builtin, "builtin.sample": user, "folder": user}}
	if got := m.ResolveViewActionOrigin("builtin.sample"); got.ExtensionID() != "" {
		t.Fatalf("shadow resolved as builtin: %#v", got)
	}
	delete(m.entries, "builtin.sample")
	if got := m.ResolveViewActionOrigin("builtin.sample"); got.ExtensionID() != "builtin.sample" {
		t.Fatalf("builtin origin: %#v", got)
	}
	for _, id := range []string{"folder", "builtin.lookalike", "missing"} {
		if got := m.ResolveViewActionOrigin(id); got != (ViewActionOrigin{}) {
			t.Fatalf("external or unknown %q: %#v", id, got)
		}
	}
	m.entries["invalid"] = &entry{builtin: true, bundle: bundleForOrigin(strings.Repeat("a", 65))}
	if got := m.ResolveViewActionOrigin("invalid"); got.ExtensionID() != "" {
		t.Fatalf("invalid ID retained: %#v", got)
	}
	var nilManager *Manager
	if got := nilManager.ResolveViewActionOrigin("builtin.sample"); got.ExtensionID() != "" {
		t.Fatalf("nil manager: %#v", got)
	}
}

func TestViewActionOriginConsumptionAndCommandIndependence(t *testing.T) {
	origin := ViewActionOrigin{extensionID: "builtin.sample"}
	command := CommandOrigin{extensionID: "builtin.sample", commandID: "run"}
	parent := WithCommandRunOrigin(context.Background(), command)
	source := WithViewActionRunOrigin(parent, origin)
	consumed, got, marked := ConsumeViewActionOrigin(source)
	if !marked || got != origin {
		t.Fatalf("first consumption: %#v %v", got, marked)
	}
	if _, got, marked := ConsumeViewActionOrigin(consumed); marked || got != (ViewActionOrigin{}) {
		t.Fatalf("marker not shadowed: %#v %v", got, marked)
	}
	if _, got, marked := ConsumeViewActionOrigin(source); !marked || got != origin {
		t.Fatalf("source changed: %#v %v", got, marked)
	}
	commandConsumed, gotCommand, marked := ConsumeCommandOrigin(consumed)
	if !marked || gotCommand != command {
		t.Fatalf("command marker lost: %#v %v", gotCommand, marked)
	}
	if _, _, marked := ConsumeViewActionOrigin(commandConsumed); marked {
		t.Fatal("view marker revived")
	}
	commandConsumed, _, _ = ConsumeCommandOrigin(source)
	if _, got, marked := ConsumeViewActionOrigin(commandConsumed); !marked || got != origin {
		t.Fatal("command consumption changed view marker")
	}
}

func TestViewActionOriginNilAndEmpty(t *testing.T) {
	//lint:ignore SA1012 Exercise the documented nil-context fallback.
	ctx, origin, marked := ConsumeViewActionOrigin(nil)
	if ctx == nil || marked || origin.ExtensionID() != "" {
		t.Fatal("nil context")
	}
	//lint:ignore SA1012 Exercise the documented nil-context fallback.
	ctx = WithViewActionRunOrigin(nil, ViewActionOrigin{})
	if _, origin, marked = ConsumeViewActionOrigin(ctx); !marked || origin.ExtensionID() != "" {
		t.Fatal("empty origin must remain marked")
	}
}
