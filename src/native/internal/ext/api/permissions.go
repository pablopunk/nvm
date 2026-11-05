package api

// Permission kinds: the OS permissions the app asks for, in the order the OS settings list them. Each is the name a
// request or a pane open takes.
const (
	PermissionMicrophone      = "microphone"
	PermissionCamera          = "camera"
	PermissionAccessibility   = "accessibility"
	PermissionScreenRecording = "screen-recording"
	PermissionFilesAndFolders = "files-and-folders"
)

// Permission states, as the OS reports them. Unknown means the OS did not answer, and it is never allowed.
const (
	PermissionAllowed       = "allowed"
	PermissionDenied        = "denied"
	PermissionNotDetermined = "not-determined"
	PermissionUnknown       = "unknown"
)

// PermissionKinds lists every permission kind, in the order above.
func PermissionKinds() []string {
	return []string{PermissionMicrophone, PermissionCamera, PermissionAccessibility, PermissionScreenRecording, PermissionFilesAndFolders}
}

// IsPermissionKind reports whether kind names one of the permission kinds.
func IsPermissionKind(kind string) bool {
	for _, candidate := range PermissionKinds() {
		if kind == candidate {
			return true
		}
	}
	return false
}

// PermissionStatus is one OS permission the platform lists, with the state the OS reports. Prompt says Enter asks the
// OS for the permission (RequestPermission) instead of opening its pane (OpenPermissionPane). The listed permissions
// are the ones that apply here, and the ones that need attention come first.
type PermissionStatus struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	State       string `json:"state"`
	CanRequest  bool   `json:"canRequest"`
	Prompt      bool   `json:"prompt"`
}
