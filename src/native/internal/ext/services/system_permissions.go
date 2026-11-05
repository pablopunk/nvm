package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// PermissionSurface is the host's side of the OS permissions: the statuses the platform lists, and the request and the
// pane that act on one kind. Its errors are the sentences an extension shows.
type PermissionSurface interface {
	Statuses() []api.PermissionStatus
	Request(kind string) error
	OpenPane(kind string) error
}

// permissionStatuses lists the permissions of the platform. Without a surface the host has none to report.
func (h *Handler) permissionStatuses() []api.PermissionStatus {
	if h.deps.Permissions == nil {
		return nil
	}
	return h.deps.Permissions.Statuses()
}

// requestPermission asks the OS to show its prompt for a permission kind. It needs the system declaration.
func (h *Handler) requestPermission(raw json.RawMessage) error {
	if err := h.require(api.CapabilitySystem); err != nil {
		return err
	}
	kind, err := permissionKindArg(raw)
	if err != nil {
		return err
	}
	if h.deps.Permissions == nil {
		return unavailable("permissions are not available on this system")
	}
	return h.deps.Permissions.Request(kind)
}

// openPermissionPane opens the System Settings pane that reviews a permission kind. It needs the system declaration.
func (h *Handler) openPermissionPane(raw json.RawMessage) error {
	if err := h.require(api.CapabilitySystem); err != nil {
		return err
	}
	kind, err := permissionKindArg(raw)
	if err != nil {
		return err
	}
	if h.deps.Permissions == nil {
		return unavailable("permissions are not available on this system")
	}
	return h.deps.Permissions.OpenPane(kind)
}

// permissionKindArg decodes a permission kind and refuses a name the API does not list.
func permissionKindArg(raw json.RawMessage) (string, error) {
	var args protocol.PermissionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if !api.IsPermissionKind(args.Kind) {
		return "", invalid("there is no permission %q", args.Kind)
	}
	return args.Kind, nil
}
