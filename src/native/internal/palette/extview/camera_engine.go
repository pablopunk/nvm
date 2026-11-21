package extview

import "fmt"

// Camera states, as the palette shows them: idle is a paused view (not visible, or not started yet).
const (
	cameraIdle     = "idle"
	cameraStarting = "starting"
	cameraLive     = "live"
	cameraError    = "error"
)

// Copy the palette shows when a live feed fails after it started.
const (
	cameraDisconnectedMessage = "Camera disconnected. Check the connection, then reopen this view."
	cameraPlaybackMessage     = "Camera feed stopped with a playback error."
)

// Where a camera selection came from. A saved device that has gone missing falls back to the default camera; an
// explicit one never does.
const (
	cameraSourceExplicit = "explicit"
	cameraSourceSaved    = "saved"
	cameraSourceDefault  = "default"
)

// cameraSelection is the device a view asks for, and where the request came from.
type cameraSelection struct {
	DeviceID string
	Source   string
}

// cameraDevice is one video input as the page enumerated it.
type cameraDevice struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
}

// cameraState is what the palette draws: the status, the reason of an error, and the device that is live.
type cameraState struct {
	Status   string
	Message  string
	DeviceID string
	Label    string
	Playing  bool
}

// cameraEvent is one report of the camera page. ID is the generation of the start the report belongs to, so a
// report from a stream that was replaced arrives as stale and is ignored.
type cameraEvent struct {
	Event    string
	ID       int
	DeviceID string
	Label    string
	Name     string
	Message  string
	Devices  []cameraDevice
}

// cameraEngine is the capture state machine of one camera view: it decides what the page starts, stops and
// forgets, and turns the page's reports into the state the palette draws. It runs on the UI thread only.
type cameraEngine struct {
	send       func(command any)
	access     func() string
	forget     func()
	onState    func()
	generation int
	selection  cameraSelection
	state      cameraState
	devices    []cameraDevice
}

// cameraCommand is an instruction for the page: start a stream with an id, or stop the current one.
type cameraCommand struct {
	Op       string `json:"op"`
	ID       int    `json:"id,omitempty"`
	DeviceID string `json:"deviceId,omitempty"`
}

// Start replaces any stream with one for selection. A system refusal from access stops before the page is asked for
// a device, so nothing starts while access is denied.
func (e *cameraEngine) Start(selection cameraSelection) {
	e.generation++
	e.selection = selection
	e.send(cameraCommand{Op: "stop"})
	e.setState(cameraState{Status: cameraStarting})
	if message := e.access(); message != "" {
		e.setState(cameraState{Status: cameraError, Message: message})
		return
	}
	e.send(cameraCommand{Op: "start", ID: e.generation, DeviceID: selection.DeviceID})
}

// Stop releases the stream, if any, and shows the view paused.
func (e *cameraEngine) Stop() {
	e.generation++
	e.send(cameraCommand{Op: "stop"})
	e.setState(cameraState{Status: cameraIdle, DeviceID: e.state.DeviceID})
}

// Handle applies one report of the page.
func (e *cameraEngine) Handle(event cameraEvent) {
	if event.Event == "devices" {
		e.devices = event.Devices
		return
	}
	if event.ID != e.generation {
		return
	}
	switch event.Event {
	case "live":
		e.setState(cameraState{Status: cameraLive, DeviceID: event.DeviceID, Label: event.Label})
	case "playing":
		state := e.state
		state.Playing = true
		e.setState(state)
	case "error":
		e.startError(event)
	case "ended":
		e.failStream(cameraDisconnectedMessage)
	case "playbackError":
		e.failStream(cameraPlaybackMessage)
	}
}

// startError handles a stream that never started. A saved device the system no longer has is forgotten and the
// default camera is tried once; any other failure is shown with its reason.
func (e *cameraEngine) startError(event cameraEvent) {
	if e.selection.Source == cameraSourceSaved && isStaleCameraDeviceError(event.Name) {
		e.forget()
		e.Start(cameraSelection{Source: cameraSourceDefault})
		return
	}
	e.setState(cameraState{Status: cameraError, Message: cameraErrorMessage(event.Name, event.Message)})
}

// failStream ends a live stream that broke: the page releases it, and the view shows why.
func (e *cameraEngine) failStream(message string) {
	e.generation++
	e.send(cameraCommand{Op: "stop"})
	e.setState(cameraState{Status: cameraError, Message: message})
}

func (e *cameraEngine) setState(state cameraState) {
	e.state = state
	e.onState()
}

// isStaleCameraDeviceError reports whether a failure names a device that is not there any more.
func isStaleCameraDeviceError(name string) bool {
	return name == "NotFoundError" || name == "OverconstrainedError"
}

// cameraErrorMessage is the copy for a failed start, from the DOM error name the page reports.
func cameraErrorMessage(name, message string) string {
	switch name {
	case "NotAllowedError":
		return "Camera access is denied. Allow Nevermind in system privacy settings."
	case "NotReadableError":
		return "The camera is busy. Close other apps using it, then reopen this view."
	case "NotSupportedError":
		return "This system web view cannot use the camera."
	}
	if isStaleCameraDeviceError(name) {
		return "The camera is not available. Check that it is connected, or choose another camera."
	}
	if message != "" {
		return message
	}
	return "Camera unavailable."
}

// nextCameraDeviceID picks the device after or before current, wrapping at the ends. With fewer than two devices
// there is nothing to switch to; an unknown current device starts from the first (or the last, going back).
func nextCameraDeviceID(devices []cameraDevice, current string, direction string) string {
	if len(devices) < 2 {
		return ""
	}
	index := -1
	for i, device := range devices {
		if device.DeviceID == current {
			index = i
			break
		}
	}
	offset := 1
	if direction == "previous" {
		offset = -1
	}
	base := -1
	if offset < 0 {
		base = len(devices)
	}
	if index >= 0 {
		base = index
	}
	count := len(devices)
	return devices[((base+offset)%count+count)%count].DeviceID
}

// cameraSwitchTitle is the label of the switch button: the active camera's name when it has one.
func cameraSwitchTitle(label string) string {
	if label == "" {
		return "Switch Camera"
	}
	return fmt.Sprintf("Switch Camera · %s", label)
}
