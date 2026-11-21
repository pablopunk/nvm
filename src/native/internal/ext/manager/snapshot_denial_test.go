package manager

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"testing"
)

func TestProtectedSnapshotDenialPreservesMutationSignal(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	h.m.mu.Lock()
	e := h.m.entries["acme.settings"]
	e.runtime = runCallOverride{runner: e.runtime, call: func(context.Context, string, string, *api.Invocation) (protocol.Response, error) {
		return protocol.Response{Data: []byte(`{"dataChanged":true,"view":{"kind":"list","title":"private"}}`), PermissionFailure: &api.Failure{Code: api.CodePermission, Message: "denied"}}, nil
	}}
	h.m.mu.Unlock()
	result, failure, err := h.m.Run(context.Background(), "acme.settings", api.Invocation{ProtectSnapshot: true})
	if err != nil || failure != nil || !result.DataChanged || result.View == nil {
		t.Fatalf("protected result=%+v failure=%+v err=%v", result, failure, err)
	}
	result, failure, err = h.m.Run(context.Background(), "acme.settings", api.Invocation{})
	if err != nil || failure != nil || result.View == nil || !result.DataChanged {
		t.Fatalf("ordinary result=%+v failure=%+v err=%v", result, failure, err)
	}
}
