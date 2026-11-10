// Package permissions is the OS Permissions built-in: the permissions the app asks for, each with the state the OS
// reports. Enter asks the OS for a permission that can be asked for, and opens its System Settings pane otherwise. The
// root's OS Permissions command opens its page.
//
// Layer: ext. The package imports api and the standard library.
package permissions

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.permissions"

// openAction is the action the root's OS Permissions command runs to get the page.
const openAction = "open"

// Refs of the row actions. Each carries the permission kind it acts on.
const (
	refRequest  = "request"
	refOpenPane = "open-pane"
	argKind     = "kind"
)

// viewTitle and placeholder name the page, as the native page named it.
const (
	viewTitle   = "Nevermind OS Permissions"
	placeholder = "Search permissions"
)

// Extension is the OS Permissions built-in. It keeps no state: the OS reports every state.
type Extension struct{}

// New returns the OS Permissions built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Its actions read and change the system's permissions, which the system declaration covers.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "OS Permissions"},
		Capabilities: []api.Capability{api.CapabilitySystem},
	}
}

// RunAction opens the page, asks the OS for a permission, or opens its pane.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case openAction:
		view := permissionsView(ctx)
		return api.Result{View: &view}, nil
	case refRequest:
		return requestResult(ctx, in.Ref.Args[argKind]), nil
	case refOpenPane:
		return paneResult(ctx, in.Ref.Args[argKind]), nil
	}
	return api.Result{}, fmt.Errorf("the OS Permissions built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// permissionsView lists the permissions the platform has, in the order the system service gives them.
func permissionsView(ctx api.Context) api.View {
	statuses := ctx.System().Permissions()
	items := make([]api.Item, 0, len(statuses))
	for _, status := range statuses {
		items = append(items, permissionItem(status))
	}
	view := api.ListView(viewTitle, items...)
	view.Placeholder = placeholder
	view.EmptyView = &api.EmptyState{Title: "No permissions", Subtitle: "No OS permissions apply on this platform."}
	return view
}

// permissionItem is the row of one permission: its state as the subtitle, the prompt or the pane as Enter, and the pane
// as the second action when Enter prompts.
func permissionItem(status api.PermissionStatus) api.Item {
	open := api.Run(refOpenPane, "Open "+status.Title+" Settings", map[string]string{argKind: status.Kind})
	open.DismissAfterRun = true
	item := api.Item{ID: "os-permission:" + status.Kind, Title: status.Title, Subtitle: permissionSubtitle(status), Icon: status.Icon}
	if !status.Prompt {
		item.Primary = &open
		return item
	}
	allow := api.Run(refRequest, "Allow "+status.Title, map[string]string{argKind: status.Kind})
	item.Primary = &allow
	item.Actions = []api.Action{open}
	return item
}

// permissionSubtitle is the sentence under a permission. It says what Enter does in the state the OS reports.
func permissionSubtitle(status api.PermissionStatus) string {
	switch status.State {
	case api.PermissionAllowed:
		return "Allowed — press Enter to review in Settings"
	case api.PermissionDenied:
		return "Not allowed — press Enter to open Settings"
	case api.PermissionNotDetermined:
		return "Not asked yet — press Enter to allow"
	}
	if status.Kind == api.PermissionAccessibility {
		return "Could not check access — press Enter to open Settings"
	}
	return "Review in Settings — press Enter to open"
}

// requestResult asks the OS for a permission and shows the list again. A refused request says where to go instead.
func requestResult(ctx api.Context, kind string) api.Result {
	result := api.Result{Navigation: api.NavReplace}
	if err := ctx.System().RequestPermission(kind); err != nil {
		result.Toast = &api.Toast{Message: err.Error(), Tone: "error"}
	}
	view := permissionsView(ctx)
	result.View = &view
	return result
}

// paneResult opens the System Settings pane of a permission. A failure is a toast.
func paneResult(ctx api.Context, kind string) api.Result {
	if err := ctx.System().OpenPermissionPane(kind); err != nil {
		return api.Result{Toast: &api.Toast{Message: err.Error(), Tone: "error"}}
	}
	return api.Result{}
}
