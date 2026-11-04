package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macPermissions reads and requests the macOS privacy permissions the window helper can check. Accessibility is the
// only one wired: the others stay unsupported, so their rows open the settings pane as before.
type macPermissions struct {
	access macHelper
}

var _ platform.Permissions = macPermissions{}

// State reports whether Accessibility is granted, without showing a prompt. A helper that cannot answer is an error,
// so the row says it could not check access.
func (p macPermissions) State(kind platform.PermissionKind) (platform.PermissionState, error) {
	if kind != platform.PermissionAccessibility {
		return platform.PermissionUnknown, platform.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), macosCallTimeout)
	defer cancel()
	access, err := p.access.accessState(ctx)
	if err != nil {
		return platform.PermissionUnknown, err
	}
	return permissionStateOf(access), nil
}

// Request shows the system's Accessibility prompt. The user grants access in System Settings, so a refusal is not an
// error here; the settings list reads the state again.
func (p macPermissions) Request(kind platform.PermissionKind) error {
	if kind != platform.PermissionAccessibility {
		return platform.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), macosCallTimeout)
	defer cancel()
	return p.access.requestAccess(ctx)
}

// permissionStateOf maps the window access answer to the state the settings list shows.
func permissionStateOf(access platform.WindowAccess) platform.PermissionState {
	switch access {
	case platform.WindowAccessAllowed:
		return platform.PermissionAllowed
	case platform.WindowAccessDenied:
		return platform.PermissionDenied
	}
	return platform.PermissionUnknown
}
