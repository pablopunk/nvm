package platform

// PermissionKind names an access the user grants to the app: an intent the OS gates behind a privacy setting.
type PermissionKind string

// Permissions the app asks for, in the order the OS settings list them.
const (
	PermissionMicrophone      PermissionKind = "microphone"
	PermissionCamera          PermissionKind = "camera"
	PermissionAccessibility   PermissionKind = "accessibility"
	PermissionScreenRecording PermissionKind = "screen-recording"
	PermissionFilesAndFolders PermissionKind = "files-and-folders"
)

// PermissionState is what the OS says about one permission. Unknown means the OS did not answer, which is
// never shown as allowed.
type PermissionState string

// Permission states.
const (
	PermissionAllowed       PermissionState = "allowed"
	PermissionDenied        PermissionState = "denied"
	PermissionNotDetermined PermissionState = "not-determined"
	PermissionUnknown       PermissionState = "unknown"
)

// Permissions is the intent "tell whether the app has a privacy permission, and ask for it when the OS allows
// a prompt". A system that cannot report a permission returns ErrUnsupported from State, which callers show as
// unknown. Request returns ErrUnsupported where the OS has no prompt; the caller then opens the settings pane.
type Permissions interface {
	// State reports the permission's current state.
	State(kind PermissionKind) (PermissionState, error)
	// Request shows the OS prompt for the permission, when the OS has one.
	Request(kind PermissionKind) error
}
