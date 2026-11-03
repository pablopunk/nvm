package real

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// trayHost creates tray items through MyGo.
type trayHost struct{}

// NewTray creates a tray item with its title, tooltip and icon.
func (trayHost) NewTray(opts platform.TrayOptions) (platform.Tray, error) {
	tray, err := mygo.NewTray(mygo.TrayOptions{ID: opts.ID, Title: opts.Title, ToolTip: opts.ToolTip, Icon: opts.Icon})
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

// convertMenuItem maps one row, wiring its action to MyGo's click callback.
func convertMenuItem(item platform.MenuItem) *mygo.MenuItem {
	action := item.Action
	return &mygo.MenuItem{
		ID:       item.ID,
		Label:    item.Label,
		Disabled: item.Disabled,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if action != nil {
				action()
			}
		},
	}
}
