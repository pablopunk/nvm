package services

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

func TestUnknownServiceAndOperationAreInvalidInput(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	requireCode(t, h.Serve(context.Background(), protocol.ServiceCall{Service: "teleport", Op: "get"}), api.CodeInvalid, "teleport")
	requireCode(t, serve(t, h, protocol.ServiceStorage, "explode", nil), api.CodeInvalid, "explode")
}

func TestBadArgumentsAreInvalidInputNotAPanic(t *testing.T) {
	h := handlerFor(t, newRig(t), manifestFor("acme.one"))
	reply := h.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceStorage, Op: protocol.OpSet, Args: []byte(`{"key": 5}`)})
	requireCode(t, reply, api.CodeInvalid, "")
	reply = h.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceStorage, Op: protocol.OpGet, Args: []byte(`not json`)})
	requireCode(t, reply, api.CodeInvalid, "")
}

func TestUndeclaredCapabilityIsRefusedWithItsName(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, manifestFor("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceSecrets, protocol.OpGet, protocol.KeyArgs{Key: "TOKEN"}), api.CodePermission, "secrets")
	requireCode(t, serve(t, h, protocol.ServiceNetwork, protocol.OpFetch, map[string]string{"url": "https://example.com"}), api.CodePermission, "network")
	if n := len(r.network.Requests()); n != 0 {
		t.Fatalf("a refused fetch reached the network %d times", n)
	}
}

func TestClosedHandlerAnswersUnavailableAndCanBeClosedTwice(t *testing.T) {
	h := New(Deps{Files: real.NewPorts().FS, Clock: fake.NewClock(), DataRoot: t.TempDir()}, manifestFor("acme.one"))
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	requireCode(t, serve(t, h, protocol.ServiceStorage, protocol.OpGet, protocol.KeyArgs{Key: "k"}), api.CodeUnavailable, "closed")
}
