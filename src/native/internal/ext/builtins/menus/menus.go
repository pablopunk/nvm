// Package menus provides the current application's menu through the extension API.
package menus

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

const ID = "nevermind.menus"
const CommandID = "search-menu-items"
const RefreshID = "refresh"
const invokeID = "invoke"

type Extension struct{ available bool }

// New declares whether the host has a menu capability; the host owns platform detection.
func New(available bool) Extension { return Extension{available: available} }

func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "Menu Items"}, Capabilities: []api.Capability{api.CapabilityDesktopMenus}}
}

func (e Extension) Commands(api.Context) ([]api.Command, error) {
	if !e.available {
		return nil, nil
	}
	return []api.Command{{ID: CommandID, Title: "Search Menu Items", Keywords: []string{"app", "menu", "current", "application"}, Section: search.KindCommands}}, nil
}

func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case CommandID:
		snapshot, err := ctx.Desktop().Menus().Snapshot()
		view := menuView(snapshot, err)
		return ctx.Navigation().Push(&view), nil
	case RefreshID:
		snapshot, err := ctx.Desktop().Menus().Snapshot()
		view := menuView(snapshot, err)
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: view.Items, IsLoading: &view.IsLoading, EmptyView: view.EmptyView}}, nil
	case invokeID:
		if err := ctx.Desktop().Menus().Invoke(in.Ref.Args["handle"]); err != nil {
			return api.Result{}, fmt.Errorf("could not run menu item: %w", err)
		}
		return api.Result{}, nil
	default:
		return api.Result{}, fmt.Errorf("unknown menu action: %w", api.ErrInvalidInput)
	}
}

func menuView(snapshot api.MenuSnapshot, err error) api.View {
	items := make([]api.Item, 0, len(snapshot.Items))
	for _, entry := range snapshot.Items {
		row := api.Item{ID: entry.Handle, Title: entry.Title, Subtitle: strings.Join(entry.Path, " › "), Keywords: entry.Path, Disabled: !entry.Enabled}
		if entry.Checked {
			row.Accessories = append(row.Accessories, api.Accessory{Icon: "check", Tooltip: "Checked"})
		}
		if entry.Shortcut != "" {
			row.Accessories = append(row.Accessories, api.Accessory{Text: entry.Shortcut})
		}
		if entry.Enabled {
			action := api.Run(invokeID, "Run Menu Item", map[string]string{"handle": entry.Handle})
			row.Primary = &action
		}
		items = append(items, row)
	}
	view := api.ListView("Menu Items", items...)
	view.Placeholder = "Search Menu Items"
	view.IsLoading = snapshot.Loading
	view.Refresh = &api.Refresh{Immediate: true, Mode: api.PatchReplace, Ref: &api.ActionRef{ID: RefreshID}}
	view.EmptyView = &api.EmptyState{Title: "No menu items found", Subtitle: "The current application has no available menu items."}
	if errors.Is(err, api.ErrPermission) {
		view.EmptyView = &api.EmptyState{Title: "Accessibility access needed", Subtitle: "Allow Nevermind in Accessibility settings to search menu items."}
	} else if err != nil {
		view.EmptyView = &api.EmptyState{Title: "Menu items unavailable", Subtitle: "Try again when the application menu is available."}
	}
	return view
}
