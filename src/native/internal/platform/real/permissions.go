package real

import "github.com/pablopunk/nvm/native/internal/platform"

// permissions reports no privacy permission. It is the port of every OS without a permission reader, so every
// permission answers Unknown, no prompt is shown, and the settings view opens the matching settings pane. Nothing
// here is reported as allowed.
type permissions struct{}

var _ platform.Permissions = permissions{}

// State reports that the OS state cannot be read.
func (permissions) State(platform.PermissionKind) (platform.PermissionState, error) {
	return platform.PermissionUnknown, platform.ErrUnsupported
}

// Request reports that no prompt is available.
func (permissions) Request(platform.PermissionKind) error {
	return platform.ErrUnsupported
}
