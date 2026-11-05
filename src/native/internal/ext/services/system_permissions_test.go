package services

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakePermissions is the host's permission surface with one fixed list of statuses. It records each request and pane.
type fakePermissions struct {
	statuses   []api.PermissionStatus
	requestErr error
	requested  []string
	opened     []string
}

func (f *fakePermissions) Statuses() []api.PermissionStatus { return f.statuses }

func (f *fakePermissions) Request(kind string) error {
	f.requested = append(f.requested, kind)
	return f.requestErr
}

func (f *fakePermissions) OpenPane(kind string) error {
	f.opened = append(f.opened, kind)
	return nil
}

var microphoneStatus = api.PermissionStatus{Kind: api.PermissionMicrophone, Title: "Microphone", State: api.PermissionNotDetermined, CanRequest: true, Prompt: true}

func permissionHandler(t *testing.T, surface PermissionSurface, capabilities ...api.Capability) *Handler {
	t.Helper()
	_, deps := newDesktopHost(t)
	deps.Permissions = surface
	return desktopHandler(t, deps, capabilities...)
}

func permissionCall(t *testing.T, h *Handler, op string, kind string) protocol.ServiceReply {
	t.Helper()
	return serve(t, h, protocol.ServiceSystem, op, protocol.PermissionArgs{Kind: kind})
}

func TestPermissionStatusesAreReadWithoutTheSystemDeclaration(t *testing.T) {
	h := permissionHandler(t, &fakePermissions{statuses: []api.PermissionStatus{microphoneStatus}})
	var got []api.PermissionStatus
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceSystem, protocol.OpPermissions, nil)), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0] != microphoneStatus {
		t.Fatalf("statuses = %+v, want the microphone status", got)
	}
}

func TestPermissionStatusesAreEmptyWithoutASurface(t *testing.T) {
	h := permissionHandler(t, nil)
	var got []api.PermissionStatus
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceSystem, protocol.OpPermissions, nil)), &got); err != nil || len(got) != 0 {
		t.Fatalf("statuses = %+v, %v; want none", got, err)
	}
}

func TestRequestNeedsTheSystemDeclaration(t *testing.T) {
	surface := &fakePermissions{}
	h := permissionHandler(t, surface)
	requireCode(t, permissionCall(t, h, protocol.OpRequestPermission, api.PermissionMicrophone), api.CodePermission, "system")
	if len(surface.requested) != 0 {
		t.Fatalf("an undeclared request reached the surface: %v", surface.requested)
	}
}

func TestOpenPaneNeedsTheSystemDeclaration(t *testing.T) {
	surface := &fakePermissions{}
	h := permissionHandler(t, surface)
	requireCode(t, permissionCall(t, h, protocol.OpOpenPermissionPane, api.PermissionCamera), api.CodePermission, "system")
	if len(surface.opened) != 0 {
		t.Fatalf("an undeclared pane open reached the surface: %v", surface.opened)
	}
}

func TestRequestAndPaneForwardAListedKind(t *testing.T) {
	surface := &fakePermissions{}
	h := permissionHandler(t, surface, api.CapabilitySystem)
	mustOK(t, permissionCall(t, h, protocol.OpRequestPermission, api.PermissionMicrophone))
	mustOK(t, permissionCall(t, h, protocol.OpOpenPermissionPane, api.PermissionCamera))
	if len(surface.requested) != 1 || surface.requested[0] != api.PermissionMicrophone {
		t.Fatalf("requested = %v, want microphone", surface.requested)
	}
	if len(surface.opened) != 1 || surface.opened[0] != api.PermissionCamera {
		t.Fatalf("opened = %v, want camera", surface.opened)
	}
}

func TestRequestRefusesAKindTheAPIDoesNotList(t *testing.T) {
	surface := &fakePermissions{}
	h := permissionHandler(t, surface, api.CapabilitySystem)
	requireCode(t, permissionCall(t, h, protocol.OpRequestPermission, "location"), api.CodeInvalid, "location")
	requireCode(t, permissionCall(t, h, protocol.OpOpenPermissionPane, ""), api.CodeInvalid, "permission")
	if len(surface.requested) != 0 || len(surface.opened) != 0 {
		t.Fatalf("an unlisted kind reached the surface: %v %v", surface.requested, surface.opened)
	}
}

func TestRequestFailureReachesTheExtensionWithItsSentence(t *testing.T) {
	surface := &fakePermissions{requestErr: errors.New("Allow microphone in System Settings.")}
	h := permissionHandler(t, surface, api.CapabilitySystem)
	failure := failureCode(t, permissionCall(t, h, protocol.OpRequestPermission, api.PermissionMicrophone))
	if failure.Message != "Allow microphone in System Settings." {
		t.Fatalf("failure = %+v, want the surface's sentence", failure)
	}
}

func TestRequestWithoutASurfaceIsUnavailable(t *testing.T) {
	h := permissionHandler(t, nil, api.CapabilitySystem)
	requireCode(t, permissionCall(t, h, protocol.OpRequestPermission, api.PermissionMicrophone), api.CodeUnavailable, "permissions")
}
