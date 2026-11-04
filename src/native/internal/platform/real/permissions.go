package real

import "github.com/pablopunk/nvm/native/internal/platform"

// permissions reports no privacy permission yet. Every OS answers Unknown and no prompt is shown, so the
// settings view says it could not check access and opens the matching settings pane. Reading the OS state
// needs the native helper on macOS and the platform's own APIs elsewhere; until those land, nothing here is
// reported as allowed.
type permissions struct{}

var _ platform.Permissions = permissions{}

// State reports that the OS state cannot be read yet.
func (permissions) State(platform.PermissionKind) (platform.PermissionState, error) {
	return platform.PermissionUnknown, platform.ErrUnsupported
}

// Request reports that no prompt is available yet.
func (permissions) Request(platform.PermissionKind) error {
	return platform.ErrUnsupported
}
