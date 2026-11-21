package services

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeUpdates is the host's updates surface with one fixed state.
type fakeUpdates struct {
	state api.UpdateState
}

func (f fakeUpdates) State() api.UpdateState { return f.state }

func updatesHandler(t *testing.T, surface UpdateSurface) *Handler {
	t.Helper()
	_, deps := newDesktopHost(t)
	deps.Updates = surface
	return desktopHandler(t, deps, api.CapabilityUpdates)
}

func TestUpdateStateIsReadWithTheUpdatesCapability(t *testing.T) {
	state := api.UpdateState{Status: api.UpdateStatusAvailable, Channel: api.UpdateChannelNightly, AvailableVersion: "2.0.1", Message: "Nevermind 2.0.1 is available"}
	h := updatesHandler(t, fakeUpdates{state: state})
	var got api.UpdateState
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceUpdates, protocol.OpState, nil)), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != state {
		t.Fatalf("state = %+v, want %+v", got, state)
	}
}

func TestUpdateStateWithoutASurfaceIsUnavailable(t *testing.T) {
	h := updatesHandler(t, nil)
	requireCode(t, serve(t, h, protocol.ServiceUpdates, protocol.OpState, nil), api.CodeUnavailable, "updates")
}

func TestUpdatesHasOnlyTheStateOperation(t *testing.T) {
	h := updatesHandler(t, fakeUpdates{})
	requireCode(t, serve(t, h, protocol.ServiceUpdates, "check", nil), api.CodeInvalid, "updates")
}

func TestUpdateStateNeedsTheUpdatesCapability(t *testing.T) {
	_, deps := newDesktopHost(t)
	deps.Updates = fakeUpdates{}
	h := desktopHandler(t, deps)
	requireCode(t, serve(t, h, protocol.ServiceUpdates, protocol.OpState, nil), api.CodePermission, "updates")
}
