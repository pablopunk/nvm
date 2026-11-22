package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteCameras lists the host's cameras. Listing never turns a camera on.
type remoteCameras struct {
	link *hostLink
}

func (c remoteCameras) Devices() ([]api.CameraDevice, error) {
	var devices []api.CameraDevice
	err := c.link.Call(protocol.ServiceCameras, protocol.OpDevices, nil, &devices)
	return devices, err
}
