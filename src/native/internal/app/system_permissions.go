package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// permissionsView lists the OS permissions the app asks for. Enter asks the OS where it can prompt, and opens the
// settings pane where it cannot; the list is read again after every change.
func (p *App) permissionsView() *palette.View {
	return &palette.View{
		Title: "Nevermind OS Permissions", Placeholder: "Search permissions",
		EmptyTitle: "No permissions", EmptySub: "No OS permissions apply on this platform.",
		Search: func(q string) []search.Item {
			var items []search.Item
			for _, row := range p.systemSvc.PermissionRows() {
				if q != "" && search.Score(row.Title, strings.ToLower(q)) == 0 {
					continue
				}
				items = append(items, p.permissionItem(row))
			}
			return items
		},
	}
}

// permissionItem is one permission row: its state as the subtitle, the prompt or the settings pane as Enter, and the
// settings pane as the second action when Enter prompts.
func (p *App) permissionItem(row systemfeat.PermissionRow) search.Item {
	openSettings := search.Action{Title: "Open " + row.Title + " Settings", Run: func(h search.Host) {
		h.Hide()
		p.openPermissionPane(row)
	}}
	if !row.NeedsPrompt() {
		return search.Item{ID: "os-permission:" + string(row.Kind), Title: row.Title, Subtitle: row.Subtitle(), Icon: row.Icon,
			Actions: []search.Action{openSettings}}
	}
	prompt := search.Action{Title: "Allow " + row.Title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		p.requestPermission(row, h)
	}}
	return search.Item{ID: "os-permission:" + string(row.Kind), Title: row.Title, Subtitle: row.Subtitle(), Icon: row.Icon,
		Actions: []search.Action{prompt, openSettings}}
}

// requestPermission asks the OS for a permission off the UI thread, then re-reads the list. A refused request says
// where to go instead.
func (p *App) requestPermission(row systemfeat.PermissionRow, h search.Host) {
	safego.Go("system.permission", func(context.Context) {
		err := p.systemSvc.RequestPermission(row.Kind)
		if err != nil {
			p.toastFromWorker(permissionRequestFailure(row.Title, err))
		}
		if p.panel != nil {
			p.panel.Update(func() { h.Refresh() })
		}
	})
}

// openPermissionPane opens the settings pane that reviews a permission.
func (p *App) openPermissionPane(row systemfeat.PermissionRow) {
	safego.Go("system.permission-pane", func(context.Context) {
		err := p.systemSvc.OpenPane(systemfeat.Pane{ID: row.Pane}, row.Anchor)
		if err != nil {
			p.toastFromWorker(describeSystemFailure(row.Title+" Settings", err))
		}
	})
}

// permissionRequestFailure says why the OS did not show a prompt, and what the user can do instead.
func permissionRequestFailure(title string, err error) string {
	if errors.Is(err, platform.ErrUnsupported) {
		return fmt.Sprintf("Allow %s in System Settings. Nevermind cannot show that prompt here.", strings.ToLower(title))
	}
	return fmt.Sprintf("Could not allow %s: %s", strings.ToLower(title), err)
}
