package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// serveCameras answers the camera list. It needs the camera capability, the same one camera views declare.
func (h *Handler) serveCameras(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityCamera); err != nil {
		return nil, err
	}
	if call.Op != protocol.OpDevices {
		return nil, invalid("cameras has no operation %q", call.Op)
	}
	if !h.camerasListable() {
		return nil, unavailable("this system cannot list its cameras")
	}
	devices, err := h.deps.Cameras.Devices()
	if err != nil {
		return nil, unsupportedAsUnavailable(err, "this system cannot list its cameras")
	}
	return cameraDevicesForExtensions(devices), nil
}

// camerasListable reports whether the host has a camera list wired on a system that supports it.
func (h *Handler) camerasListable() bool {
	return h.deps.Cameras != nil && h.deps.Capabilities.Supported(platform.CapabilityCameraDevices)
}

// cameraDevicesForExtensions puts the system default first and keeps the OS order for the rest.
func cameraDevicesForExtensions(devices []platform.CameraDevice) []api.CameraDevice {
	out := make([]api.CameraDevice, 0, len(devices))
	for _, device := range devices {
		converted := api.CameraDevice{ID: device.ID, Title: device.Title, IsDefault: device.Default, Kind: api.CameraKind(device.Kind)}
		if device.Default {
			out = append([]api.CameraDevice{converted}, out...)
			continue
		}
		out = append(out, converted)
	}
	return out
}
