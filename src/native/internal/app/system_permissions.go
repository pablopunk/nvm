package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/permissions"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// permissionsPage is the OS permissions page the root's OS Permissions command opens. The OS Permissions built-in
// declares the rows, and the palette draws them.
func (p *App) permissionsPage() *palette.View {
	fallback := &palette.View{Title: "Nevermind OS Permissions", Placeholder: "Search permissions", Search: func(string) []search.Item { return nil }}
	return p.builtInOpenPage(permissions.ID, fallback)
}

// permissionSurface serves the OS permissions to the extension API: the rows the system service lists, and the request
// and the pane of one kind.
type permissionSurface struct {
	p *App
}

// Statuses lists the permissions of the platform, the ones that need attention first.
func (s permissionSurface) Statuses() []api.PermissionStatus {
	rows := s.p.systemSvc.PermissionRows()
	statuses := make([]api.PermissionStatus, 0, len(rows))
	for _, row := range rows {
		statuses = append(statuses, apiPermissionStatus(row))
	}
	return statuses
}

// Request asks the OS for a permission kind. A refused request says where to go instead.
func (s permissionSurface) Request(kind string) error {
	row, ok := s.p.systemPermissionRow(kind)
	if !ok {
		return fmt.Errorf("there is no permission %q on this system", kind)
	}
	if err := s.p.systemSvc.RequestPermission(platform.PermissionKind(kind)); err != nil {
		return errors.New(permissionRequestFailure(row.Title, err))
	}
	return nil
}

// OpenPane opens the settings pane that reviews a permission kind.
func (s permissionSurface) OpenPane(kind string) error {
	row, ok := s.p.systemPermissionRow(kind)
	if !ok {
		return fmt.Errorf("there is no permission %q on this system", kind)
	}
	if err := s.p.systemSvc.OpenPane(systemfeat.Pane{ID: row.Pane}, row.Anchor); err != nil {
		return errors.New(describeSystemFailure(row.Title+" Settings", err))
	}
	return nil
}

// systemPermissionRow finds the row of a permission kind among the ones this platform lists.
func (p *App) systemPermissionRow(kind string) (systemfeat.PermissionRow, bool) {
	for _, row := range p.systemSvc.PermissionRows() {
		if string(row.Kind) == kind {
			return row, true
		}
	}
	return systemfeat.PermissionRow{}, false
}

// apiPermissionStatus converts a permission row into the status the extension API reports. Prompt is the rule that
// Enter asks the OS for the permission rather than opening its pane.
func apiPermissionStatus(row systemfeat.PermissionRow) api.PermissionStatus {
	return api.PermissionStatus{
		Kind:        string(row.Kind),
		Title:       row.Title,
		Description: row.Description,
		Icon:        row.Icon,
		State:       string(row.State),
		CanRequest:  row.CanRequest,
		Prompt:      row.NeedsPrompt(),
	}
}

// permissionRequestFailure says why the OS did not show a prompt, and what the user can do instead.
func permissionRequestFailure(title string, err error) string {
	if errors.Is(err, platform.ErrUnsupported) {
		return fmt.Sprintf("Allow %s in System Settings. Nevermind cannot show that prompt here.", strings.ToLower(title))
	}
	return fmt.Sprintf("Could not allow %s: %s", strings.ToLower(title), err)
}
