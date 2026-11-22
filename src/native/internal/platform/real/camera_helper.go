package real

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/swifthelper"
)

// cameraSubcommand is the helper's camera command group. `camera devices` answers with the helper's envelope.
const cameraSubcommand = "camera"

// macCameras lists the video inputs through the Swift helper that ships with the app.
type macCameras struct {
	helper platform.Helper
	binary string
}

// cameraDeviceWire is one entry of the helper's camera list.
type cameraDeviceWire struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Default bool   `json:"default"`
	Kind    string `json:"kind"`
}

// Devices lists the video inputs and marks the system default.
func (c macCameras) Devices() ([]platform.CameraDevice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), macosCallTimeout)
	defer cancel()
	result, err := c.helper.Run(ctx, c.binary, []string{cameraSubcommand, "devices"}, nil)
	if err != nil {
		return nil, fmt.Errorf("the camera helper did not run: %w", err)
	}
	envelope, err := swifthelper.ParseEnvelope(result.Stdout)
	if err != nil {
		return nil, errors.New("the camera helper returned an unreadable response")
	}
	if envelope.Code != swifthelper.CodeOK {
		return nil, fmt.Errorf("the camera helper answered %q: %s", envelope.Code, envelope.Message)
	}
	var wire []cameraDeviceWire
	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, &wire); err != nil {
			return nil, errors.New("the camera helper returned an unreadable device list")
		}
	}
	return camerasFromWire(wire), nil
}

// camerasFromWire keeps the devices that have an ID, titled by their ID when the OS gave no name.
func camerasFromWire(wire []cameraDeviceWire) []platform.CameraDevice {
	devices := make([]platform.CameraDevice, 0, len(wire))
	for _, device := range wire {
		if device.ID == "" {
			continue
		}
		title := device.Title
		if title == "" {
			title = device.ID
		}
		devices = append(devices, platform.CameraDevice{ID: device.ID, Title: title, Default: device.Default, Kind: cameraKindOf(device.Kind)})
	}
	return devices
}

// cameraKindOf reads the helper's kind word; a word this build does not know is CameraOther.
func cameraKindOf(kind string) platform.CameraKind {
	switch known := platform.CameraKind(kind); known {
	case platform.CameraBuiltIn, platform.CameraExternal, platform.CameraContinuity, platform.CameraVirtual:
		return known
	}
	return platform.CameraOther
}
