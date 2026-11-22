package extview

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// cameraDevicesSection is the title of the camera view's action menu group that lists its cameras.
const cameraDevicesSection = "Cameras"

// Where a camera view stands with the host's camera list: still asking, listed by the host, or listed by its page
// because the host cannot list cameras.
const (
	cameraListPending = iota
	cameraListHost
	cameraListPage
)

// cameraDeviceList is the host's camera list for one camera view. A start that names a camera waits for the list, since
// only the list turns the host's camera ID into the name the page opens.
type cameraDeviceList struct {
	source        platform.Cameras
	onUI          func(func())
	state         int
	startDeferred bool
}

// start starts the capture of the selected camera, or waits for the host's list when the selection names a camera.
func (c *cameraView) start() {
	if c.devices.state == cameraListPending && c.selection.DeviceID != "" {
		c.devices.startDeferred = true
		return
	}
	c.devices.startDeferred = false
	c.engine.Start(c.selection)
}

// loadHostDevices reads the host's camera list off the UI thread. Without one, the view uses the page's list.
func (c *cameraView) loadHostDevices() {
	source := c.devices.source
	if source == nil {
		c.devices.state = cameraListPage
		return
	}
	go func() {
		devices, err := source.Devices()
		c.devices.onUI(func() { c.hostDevicesLoaded(devices, err) })
	}()
}

// hostDevicesLoaded takes the host's list, or falls back to the page's when the first read fails. A later failure keeps
// the list it has.
func (c *cameraView) hostDevicesLoaded(devices []platform.CameraDevice, err error) {
	if c.page == nil {
		return
	}
	if err != nil {
		if c.devices.state == cameraListPending {
			c.devices.state = cameraListPage
			c.resumeDeferredStart()
		}
		return
	}
	c.devices.state = cameraListHost
	c.engine.SetHostDevices(cameraDevicesOfHost(devices))
	c.resumeDeferredStart()
	c.engine.onState()
}

func (c *cameraView) resumeDeferredStart() {
	if c.devices.startDeferred && c.active {
		c.start()
	}
}

// cameraDevicesOfHost names each host camera by the OS name the page also reads.
func cameraDevicesOfHost(devices []platform.CameraDevice) []cameraDevice {
	out := make([]cameraDevice, 0, len(devices))
	for _, device := range devices {
		out = append(out, cameraDevice{DeviceID: device.ID, Label: device.Title})
	}
	return out
}

// cameraPageActions is the camera view's menu: the extension's actions, then one row per camera.
func (s *Session) cameraPageActions() []search.Action {
	out := s.pageActions()
	if s.camera != nil {
		out = append(out, s.camera.deviceActions()...)
	}
	return out
}

// deviceActions lists the cameras in the action menu when the view shows its switcher; the live one says so.
func (c *cameraView) deviceActions() []search.Action {
	if c.page == nil || !c.showsSwitcher() {
		return nil
	}
	current := c.currentDeviceID()
	out := make([]search.Action, 0, len(c.engine.devices))
	for _, device := range c.engine.devices {
		deviceID := device.DeviceID
		action := search.Action{
			Title: cameraMenuTitle(device), Section: cameraDevicesSection, MenuOnly: true, Dismiss: search.DismissNever,
			Run: func(search.Host) { c.selectDevice(deviceID) },
		}
		if deviceID == current {
			action.Subtitle = "In Use"
		}
		out = append(out, action)
	}
	return out
}

func cameraMenuTitle(device cameraDevice) string {
	if device.Label != "" {
		return device.Label
	}
	return "Camera"
}
