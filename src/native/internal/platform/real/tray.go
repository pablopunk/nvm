package real

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// trayHost creates tray items through MyGo.
type trayHost struct{}

// NewTray creates a tray item with its title and tooltip. Its icon is set afterwards with SetIcon.
func (trayHost) NewTray(opts platform.TrayOptions) (platform.Tray, error) {
	tray, err := mygo.NewTray(mygo.TrayOptions{ID: opts.ID, Title: opts.Title, ToolTip: opts.ToolTip})
	if err != nil {
		return nil, err
	}
	return &nativeTray{tray: tray}, nil
}

// nativeTray wraps a MyGo tray item.
type nativeTray struct {
	tray *mygo.Tray
}

// SetTitle changes the text next to the icon.
func (t *nativeTray) SetTitle(title string) {
	t.tray.SetTitle(title)
}

// SetMenu replaces the click menu. An empty list removes it.
func (t *nativeTray) SetMenu(items []platform.MenuItem) {
	if len(items) == 0 {
		t.tray.SetMenu(nil)
		return
	}
	t.tray.SetMenu(mygo.NewMenu(convertMenuItems(items)))
}

// SetIcon sets the PNG icon; MyGo tints a template icon to the menu bar on macOS.
func (t *nativeTray) SetIcon(png []byte, template bool) error {
	if len(png) == 0 {
		return nil
	}
	return t.tray.SetIcon(png, template)
}

// Dark reports whether the bar the icon sits on is dark.
func (t *nativeTray) Dark() bool {
	return barIsDark(t.tray)
}

// OnAppearanceChange forwards MyGo's appearance callback.
func (t *nativeTray) OnAppearanceChange(fn func()) {
	t.tray.OnAppearanceChange(fn)
}

// Close removes the item from the menu bar or tray.
func (t *nativeTray) Close() {
	t.tray.Destroy()
}

// convertMenuItems maps the port's rows onto MyGo menu items. Actions run without the window argument
// MyGo passes, which the port does not expose.
func convertMenuItems(items []platform.MenuItem) []*mygo.MenuItem {
	converted := make([]*mygo.MenuItem, 0, len(items))
	for _, item := range items {
		if item.Separator {
			converted = append(converted, mygo.Separator())
			continue
		}
		converted = append(converted, convertMenuItem(item))
	}
	return converted
}

// convertMenuItem maps one row, wiring its action to MyGo's click callback. An icon MyGo cannot decode is left out,
// so the row shows its label alone.
func convertMenuItem(item platform.MenuItem) *mygo.MenuItem {
	action := item.Action
	converted := &mygo.MenuItem{
		ID:          item.ID,
		Label:       item.Label,
		Disabled:    item.Disabled,
		Accelerator: item.Accelerator,
		Icon:        convertMenuIcon(item.Icon),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if action != nil {
				action()
			}
		},
	}
	if len(item.Submenu) > 0 {
		converted.Submenu = convertMenuItems(item.Submenu)
	}
	return converted
}

// convertMenuIcon maps a row's icon to MyGo's, or nil when there is none or MyGo cannot decode it.
func convertMenuIcon(icon *platform.MenuIcon) *mygo.MenuIcon {
	if icon == nil {
		return nil
	}
	converted, err := mygo.NewMenuIcon(icon.PNG, icon.Template)
	if err != nil {
		return nil
	}
	return converted
}
