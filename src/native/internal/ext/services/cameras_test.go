package services

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// camerasHandler returns the services of an extension over cameras, on a system with the given capabilities.
func camerasHandler(t *testing.T, cameras platform.Cameras, system platform.Capabilities, declared ...api.Capability) *Handler {
	t.Helper()
	handler := New(Deps{Clock: fake.NewClock(), DataRoot: t.TempDir(), Cameras: cameras, Capabilities: system},
		manifestFor("acme.cameras", declared...))
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

func camerasCall(handler *Handler) protocol.ServiceReply {
	return handler.Serve(context.Background(), protocol.ServiceCall{Service: protocol.ServiceCameras, Op: protocol.OpDevices})
}

func TestCamerasListTheDefaultFirstWithStableIDs(t *testing.T) {
	cameras := fake.NewCameras()
	cameras.SetDevices(
		platform.CameraDevice{ID: "usb-1", Title: "HP Display Camera", Kind: platform.CameraExternal},
		platform.CameraDevice{ID: "built-in", Title: "FaceTime HD Camera", Default: true, Kind: platform.CameraBuiltIn},
	)
	handler := camerasHandler(t, cameras, platform.NewCapabilities(platform.CapabilityCameraDevices), api.CapabilityCamera)
	var devices []api.CameraDevice
	decodeReply(t, camerasCall(handler), &devices)
	want := []api.CameraDevice{
		{ID: "built-in", Title: "FaceTime HD Camera", IsDefault: true, Kind: api.CameraBuiltIn},
		{ID: "usb-1", Title: "HP Display Camera", Kind: api.CameraExternal},
	}
	if len(devices) != len(want) || devices[0] != want[0] || devices[1] != want[1] {
		t.Fatalf("devices %+v; want %+v", devices, want)
	}
}

func TestCamerasNeedTheCameraDeclaration(t *testing.T) {
	handler := camerasHandler(t, fake.NewCameras(), platform.NewCapabilities(platform.CapabilityCameraDevices))
	if reply := camerasCall(handler); reply.Failure == nil {
		t.Fatalf("reply %+v; want a refusal without the camera declaration", reply)
	}
}

func TestCamerasAreUnavailableWhereTheSystemCannotListThem(t *testing.T) {
	handler := camerasHandler(t, fake.NewCameras(), platform.NewCapabilities(), api.CapabilityCamera)
	reply := camerasCall(handler)
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnavailable {
		t.Fatalf("reply %+v; want an unavailable failure", reply)
	}
	if handler.publicCapability("camera.devices") {
		t.Fatal("camera.devices is reported without the system capability")
	}
}
