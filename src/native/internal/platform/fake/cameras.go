package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpCamerasDevices is the operation name accepted by Cameras.FailNext.
const OpCamerasDevices = "cameras.devices"

// Cameras is a platform.Cameras with a scripted device list.
type Cameras struct {
	mu      sync.Mutex
	devices []platform.CameraDevice
	faults  faultSet
}

var _ platform.Cameras = (*Cameras)(nil)

// NewCameras returns a camera list with one default built-in camera.
func NewCameras() *Cameras {
	return &Cameras{devices: []platform.CameraDevice{{ID: "built-in", Title: "FaceTime HD Camera", Default: true, Kind: platform.CameraBuiltIn}}}
}

// SetDevices replaces the device list.
func (c *Cameras) SetDevices(devices ...platform.CameraDevice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devices = append([]platform.CameraDevice(nil), devices...)
}

// FailNext makes the next call of op return err.
func (c *Cameras) FailNext(op string, err error) {
	c.faults.failNext(op, err)
}

// Devices returns the scripted device list.
func (c *Cameras) Devices() ([]platform.CameraDevice, error) {
	if err := c.faults.take(OpCamerasDevices); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]platform.CameraDevice(nil), c.devices...), nil
}
