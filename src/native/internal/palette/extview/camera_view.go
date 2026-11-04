package extview

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// CameraMemory remembers the camera a camera view last chose, by the view's storage key, so reopening the view
// starts with the same camera.
type CameraMemory interface {
	Saved(key string) (string, bool)
	Remember(key, deviceID string)
	Forget(key string)
}

// NewMemoryCameras returns a CameraMemory that lives as long as the value does.
func NewMemoryCameras() CameraMemory {
	return &memoryCameras{saved: map[string]string{}}
}

type memoryCameras struct {
	mu    sync.Mutex
	saved map[string]string
}

func (m *memoryCameras) Saved(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.saved[key]
	return id, ok
}

func (m *memoryCameras) Remember(key, deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved[key] = deviceID
}

func (m *memoryCameras) Forget(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.saved, key)
}

// cameraView is the capture of one camera view: its page, the engine that drives the page, the device it asks for,
// and whether the palette shows it as the active page.
type cameraView struct {
	page      platform.WebPage
	engine    *cameraEngine
	key       string
	memory    CameraMemory
	selection cameraSelection
	config    cameraPageConfig
	switcher  bool
	active    bool
}

// openCamera makes the camera's page and its engine. A system without a web view leaves the page nil, and the view
// then says so; nothing starts.
func (s *Session) openCamera() {
	spec := s.view.Camera
	cam := &cameraView{
		key:    cameraStorageKey(s.view),
		memory: s.deps.Cameras,
		config: cameraPageConfig{
			Muted: spec.Muted == nil || *spec.Muted, Controls: spec.Controls,
			Mirrored: spec.Mirrored, Framing: cameraFraming(spec.Framing),
		},
		switcher: spec.ShowDeviceSwitcher == nil || *spec.ShowDeviceSwitcher,
	}
	cam.selection = initialCameraSelection(spec.DeviceID, cam.memory, cam.key)
	cam.engine = &cameraEngine{
		send:    cam.send,
		access:  func() string { return cameraAccessMessage(s.deps.Permissions) },
		forget:  func() { cam.memory.Forget(cam.key) },
		onState: s.refreshHost,
	}
	cam.engine.state.Status = cameraIdle
	s.camera = cam
	page, err := s.openPage(platform.WebPageSpec{
		Grant:        webviewGrant([]api.WebviewPermission{api.WebviewCamera}),
		Navigate:     webviewNavigationAllowed,
		OpenExternal: externalOpener(s.deps.OpenURL),
		Title:        cam.handleTitle,
	})
	if err != nil {
		return
	}
	cam.page = page
	page.Load(cameraPageHTML(cam.config))
}

// cameraStorageKey names the view's remembered camera, as the Electron view does: by its id, else its title.
func cameraStorageKey(view api.View) string {
	if view.ID != "" {
		return "camera-device:" + view.ID
	}
	return "camera-device:" + view.Title
}

// cameraFraming is cover unless the view asks for contain.
func cameraFraming(framing string) string {
	if framing == "contain" {
		return "contain"
	}
	return "cover"
}

// initialCameraSelection picks the device a view starts with: the explicit one, else the remembered one, else the
// system default.
func initialCameraSelection(explicit string, memory CameraMemory, key string) cameraSelection {
	if explicit != "" {
		return cameraSelection{DeviceID: explicit, Source: cameraSourceExplicit}
	}
	if saved, ok := memory.Saved(key); ok && saved != "" {
		return cameraSelection{DeviceID: saved, Source: cameraSourceSaved}
	}
	return cameraSelection{Source: cameraSourceDefault}
}

// cameraAccessMessage is the reason the system refuses the camera before any stream is asked for, or "" when it
// does not refuse it. A system that cannot report the permission (unknown) lets the page ask, and the system then
// prompts or refuses on its own.
func cameraAccessMessage(permissions platform.Permissions) string {
	if permissions == nil {
		return ""
	}
	state, err := permissions.State(platform.PermissionCamera)
	if err == nil && state == platform.PermissionDenied {
		return "Camera access is denied in system privacy settings."
	}
	return ""
}

func (c *cameraView) send(command any) {
	if c.page != nil {
		c.page.Run(cameraCommandScript(command))
	}
}

func (c *cameraView) handleTitle(title string) {
	if event, ok := parseCameraTitle(title); ok {
		c.engine.Handle(event)
	}
}

// setActive starts the capture while the view is the active page of a shown palette, and stops it otherwise.
func (c *cameraView) setActive(active bool) {
	if c.page == nil || active == c.active {
		return
	}
	c.active = active
	if active {
		c.engine.Start(c.selection)
		return
	}
	c.engine.Stop()
}

// close stops the capture and destroys the page, when the view leaves.
func (c *cameraView) close() {
	if c.page == nil {
		return
	}
	c.engine.Stop()
	c.page.Destroy()
	c.page = nil
}

// operate runs a camera action: switching devices, muting, or the controls.
func (c *cameraView) operate(op string) {
	switch op {
	case "switch-device", "next-device":
		c.switchDevice("next")
	case "previous-device":
		c.switchDevice("previous")
	case "toggle-muted":
		c.config.Muted = !c.config.Muted
		c.configure()
	case "toggle-controls":
		c.config.Controls = !c.config.Controls
		c.configure()
	}
}

// switchDevice moves to the next or previous camera, remembers it, and restarts the capture when it is running.
func (c *cameraView) switchDevice(direction string) {
	current := c.engine.state.DeviceID
	if current == "" {
		current = c.selection.DeviceID
	}
	deviceID := nextCameraDeviceID(c.engine.devices, current, direction)
	if deviceID == "" {
		return
	}
	c.memory.Remember(c.key, deviceID)
	c.selection = cameraSelection{DeviceID: deviceID, Source: cameraSourceSaved}
	if c.active {
		c.engine.Start(c.selection)
	}
}

func (c *cameraView) configure() {
	c.send(cameraConfigureCommand{Op: "configure", Options: c.config})
}

func (c *cameraView) showsSwitcher() bool {
	return c.switcher && len(c.engine.devices) > 1
}

// operateCamera runs a camera action of the view, or says the view has no camera.
func (s *Session) operateCamera(h search.Host, op string) {
	if s.camera == nil || s.camera.page == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.camera.operate(op)
}

// drawCamera draws the camera's page with its status over it, the error when the stream failed, and the switch
// button when the system has more than one camera.
func drawCamera(c *ui.Context, cam *cameraView) {
	if cam == nil || cam.page == nil {
		drawNotice(c, webViewUnavailableMessage)
		return
	}
	drawPage(c, cam.page, func() { drawCameraOverlay(c, cam) })
}

func drawCameraOverlay(c *ui.Context, cam *cameraView) {
	t := design.Tokens(c)
	state := cam.engine.state
	if state.Status == cameraError {
		ui.Column(c).Absolute().Top(12).Left(12).Right(12).Padding(12).Gap(4).Radius(12).Background(t.Raised).Children(func() {
			ui.Text(c, "Camera Issue").FontSize(13).TextColor(t.Strong)
			ui.Text(c, state.Message).FontSize(11).TextColor(t.Secondary)
		})
	}
	ui.Text(c, cameraStatusText(state)).Absolute().Bottom(10).Left(12).FontSize(11).TextColor(t.Secondary)
	if cam.showsSwitcher() {
		switchButton := ui.Button(c, cameraSwitchTitle(state.Label)).Absolute().Bottom(8).Right(12)
		if switchButton.Clicked() {
			cam.switchDevice("next")
		}
	}
}

// cameraStatusText is the status line of the view: what the capture is doing, or that it is paused.
func cameraStatusText(state cameraState) string {
	switch state.Status {
	case cameraStarting:
		return "Requesting camera…"
	case cameraLive:
		if state.Playing {
			return "Live"
		}
		return "Starting…"
	case cameraError:
		return "Camera unavailable"
	}
	return "Paused"
}
