package platform

// CameraKind says what a camera is, so a picker can group or label it.
type CameraKind string

const (
	CameraBuiltIn    CameraKind = "built-in"
	CameraExternal   CameraKind = "external"
	CameraContinuity CameraKind = "continuity"
	CameraVirtual    CameraKind = "virtual"
	CameraOther      CameraKind = "other"
)

// CameraDevice is a video input the user can pick. ID stays the same for the same camera across launches; Title is
// the name the OS shows, which is also the label the system web view gives the camera.
type CameraDevice struct {
	ID      string
	Title   string
	Default bool
	Kind    CameraKind
}

// Cameras is the intent "list the video inputs". It needs the capability camera.devices; where the OS cannot list
// them, Devices returns ErrUnsupported. Listing never asks for camera permission and never turns a camera on.
type Cameras interface {
	Devices() ([]CameraDevice, error)
}
