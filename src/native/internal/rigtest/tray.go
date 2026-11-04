//go:build rig

package rigtest

import (
	"fmt"
	"regexp"
	"strings"
)

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

const (
	statusNotifierItemInterface = "org.kde.StatusNotifierItem"
	dbusMenuInterface           = "com.canonical.dbusmenu"
)

// The menu reply of dbus-send --print-reply: an item's id is an int32 line, and its label is a string "label" key whose
// value is the next variant line.
var (
	menuIDLine       = regexp.MustCompile(`^\s*int32 (-?\d+)\s*$`)
	menuLabelKeyLine = regexp.MustCompile(`^\s*string "label"\s*$`)
	menuLabelLine    = regexp.MustCompile(`^\s*variant\s+string "(.*)"\s*$`)
)

// trayItem is one status notifier item: the bus name that owns it and its object path.
type trayItem struct {
	bus, path string
}

// trayItemOfApp returns the one status notifier item the watcher lists. The rig runs one app at a time, so any other
// item means the session is not the rig's own.
func (a *App) trayItemOfApp() (trayItem, error) {
	entries, err := a.TrayItems()
	if err != nil {
		return trayItem{}, err
	}
	var items []trayItem
	for _, entry := range entries {
		if bus, path, ok := strings.Cut(entry, "/"); ok {
			items = append(items, trayItem{bus: bus, path: "/" + path})
		}
	}
	if len(items) != 1 {
		return trayItem{}, fmt.Errorf("want one status notifier item, watcher lists %q", entries)
	}
	return items[0], nil
}

// trayMenuPath reads the Menu property of an item: the object path of the D-Bus menu the tray host draws.
func (a *App) trayMenuPath(item trayItem) (string, error) {
	out, err := a.output("dbus-send", "--session", "--print-reply", "--dest="+item.bus, item.path,
		"org.freedesktop.DBus.Properties.Get", "string:"+statusNotifierItemInterface, "string:Menu")
	if err != nil {
		return "", err
	}
	values := quotedStrings(out)
	if len(values) == 0 {
		return "", fmt.Errorf("the item has no Menu property: %s", out)
	}
	return values[0], nil
}

// menuRowIDs reads the menu layout and maps each row's label to the id the menu uses for it.
func (a *App) menuRowIDs(item trayItem, menuPath string) (map[string]string, error) {
	out, err := a.output("dbus-send", "--session", "--print-reply", "--dest="+item.bus, menuPath,
		dbusMenuInterface+".GetLayout", "int32:0", "int32:-1", "array:string:")
	if err != nil {
		return nil, err
	}
	return parseMenuLabels(out), nil
}

// parseMenuLabels pairs each label in a layout reply with the item id printed just before its properties.
func parseMenuLabels(reply string) map[string]string {
	labels := map[string]string{}
	currentID := ""
	awaitingLabel := false
	for _, line := range strings.Split(reply, "\n") {
		if m := menuIDLine.FindStringSubmatch(line); m != nil {
			currentID = m[1]
		}
		if menuLabelKeyLine.MatchString(line) {
			awaitingLabel = true
			continue
		}
		if awaitingLabel {
			if m := menuLabelLine.FindStringSubmatch(line); m != nil {
				labels[m[1]] = currentID
			}
			awaitingLabel = false
		}
	}
	return labels
}
