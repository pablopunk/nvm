//go:build rig

package rigtest

import "strings"

const (
	statusNotifierWatcher = "org.kde.StatusNotifierWatcher"
	trayItemsProperty     = "RegisteredStatusNotifierItems"
)

// TrayItems lists the status notifier items registered with the session's watcher, which the rig
// runs as the xfce4-panel tray host. It is best effort: when no watcher is on the bus it returns an
// error, and no journey depends on it yet (the tray is verified by its registration here, not by
// reading its icon).
func (a *App) TrayItems() ([]string, error) {
	out, err := a.output("dbus-send", "--session", "--print-reply", "--dest="+statusNotifierWatcher,
		"/StatusNotifierWatcher", "org.freedesktop.DBus.Properties.Get",
		"string:"+statusNotifierWatcher, "string:"+trayItemsProperty)
	if err != nil {
		return nil, err
	}
	return quotedStrings(out), nil
}

// quotedStrings extracts the double-quoted values of a dbus-send reply, one per item.
func quotedStrings(reply string) []string {
	var items []string
	for {
		start := strings.Index(reply, "\"")
		if start < 0 {
			return items
		}
		rest := reply[start+1:]
		end := strings.Index(rest, "\"")
		if end < 0 {
			return items
		}
		items = append(items, rest[:end])
		reply = rest[end+1:]
	}
}
