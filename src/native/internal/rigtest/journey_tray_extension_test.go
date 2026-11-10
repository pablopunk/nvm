//go:build rig

package rigtest

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// trayExtensionFixture is the menu bar fixture: a "main" item with sections, a submenu and actions that change its own
// state, and a second item. Its title is the tooltip "Menu bar fixture", which identifies its tray item.
const (
	trayExtensionID    = "acme.menubar"
	trayExtensionQuery = "menu bar"
	trayExtensionTitle = "Menu bar fixture"
	trayWait           = 20 * time.Second
)

// trayRowAction is the dbusmenu event a click sends: the row's id, "clicked", no data and a zero timestamp.
const trayRowAction = "com.canonical.dbusmenu.Event"

// TestJourneyTrayExtensionItemRegistersRunsItsRowsAndRefreshes checks the extension menu bar of WP3.5 on the rig. An
// enabled extension gets its own status item with the title, sections and submenu it declares. Clicking Bump runs the
// extension's action; the action asks for a menu refresh, so the count row changes. Not one of PLAN's journeys.
func TestJourneyTrayExtensionItemRegistersRunsItsRowsAndRefreshes(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, trayExtensionID)
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	enableThroughReview(t, app, trayExtensionQuery, trayExtensionTitle)
	hidePalette(t, app)

	item := waitForTrayItemTitled(t, app, trayExtensionTitle)
	if _, err := app.Screenshot("tray-ext-1-registered"); err != nil {
		t.Errorf("desktop screenshot: %v", err)
	}
	menuPath, err := app.trayMenuPath(item)
	must(t, err)
	rows := waitForMenuRows(t, app, item, menuPath, "Bump", "Break", "Fix", "Documentation")
	if _, ok := rows["Count 0"]; !ok {
		t.Fatalf("the counter row is not in the menu: %v", rows)
	}

	must(t, app.clickMenuRow(item, menuPath, rows["Bump"]))
	rows = waitForMenuRows(t, app, item, menuPath, "Count 1")
	if _, err := app.Screenshot("tray-ext-2-after-bump"); err != nil {
		t.Errorf("desktop screenshot: %v", err)
	}

	must(t, app.clickMenuRow(item, menuPath, rows["Break"]))
	rows = waitForMenuLabelPrefix(t, app, item, menuPath, notRespondingPrefix)
	if _, ok := rows["Bump"]; !ok {
		t.Fatalf("a failing render dropped the last good rows: %v", rows)
	}
	if _, err := app.Screenshot("tray-ext-3-not-responding"); err != nil {
		t.Errorf("desktop screenshot: %v", err)
	}

	must(t, app.clickMenuRow(item, menuPath, rows["Fix"]))
	waitForMenuLabelAbsent(t, app, item, menuPath, notRespondingPrefix)
}

// notRespondingPrefix starts the head row a menu shows while its extension fails to render.
const notRespondingPrefix = "Not responding: "

// TestJourneyTrayAppItemSettingsRowOpensSettings checks that a click on the app's own menu row reaches its action: the
// Settings row of WP1.12 opens the Settings page in the palette. It is the click the WP1.12 rig note left open.
func TestJourneyTrayAppItemSettingsRowOpensSettings(t *testing.T) {
	app := Launch(t, Options{})
	item := waitForTrayItemTitled(t, app, "Nevermind")
	menuPath, err := app.trayMenuPath(item)
	must(t, err)
	rows := waitForMenuRows(t, app, item, menuPath, "Settings…")
	must(t, app.clickMenuRow(item, menuPath, rows["Settings…"]))
	_, err = app.WaitFor("Open Nevermind Shortcut", trayWait)
	must(t, err)
}

// waitForTrayItemTitled returns the status notifier item whose title starts with title, polling the watcher until it
// registers. A status item's hover text carries its ticking title after the name, so the name is what identifies it.
func waitForTrayItemTitled(t *testing.T, app *App, title string) trayItem {
	t.Helper()
	deadline := time.Now().Add(trayWait)
	for {
		items, err := app.trayItemsWithTitles()
		if err == nil {
			for _, item := range items {
				if strings.HasPrefix(item.title, title) {
					return item.trayItem
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no tray item titled %q (last error %v, items %+v)", title, err, items)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// waitForMenuRows polls the item's menu until every label is in it, and returns the rows of the last read.
func waitForMenuRows(t *testing.T, app *App, item trayItem, menuPath string, labels ...string) map[string]string {
	t.Helper()
	deadline := time.Now().Add(trayWait)
	for {
		rows, err := app.menuRowIDs(item, menuPath)
		must(t, err)
		missing := []string{}
		for _, label := range labels {
			if _, ok := rows[label]; !ok {
				missing = append(missing, label)
			}
		}
		if len(missing) == 0 {
			return rows
		}
		if time.Now().After(deadline) {
			t.Fatalf("the menu lacks %s: %v", strings.Join(missing, ", "), rows)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// waitForMenuLabelPrefix polls the menu until a row label starts with prefix, and returns the rows of that read.
func waitForMenuLabelPrefix(t *testing.T, app *App, item trayItem, menuPath, prefix string) map[string]string {
	t.Helper()
	return pollMenu(t, app, item, menuPath, fmt.Sprintf("a row starting %q", prefix), func(rows map[string]string) bool {
		return hasLabelPrefix(rows, prefix)
	})
}

// waitForMenuLabelAbsent polls the menu until no row label starts with prefix.
func waitForMenuLabelAbsent(t *testing.T, app *App, item trayItem, menuPath, prefix string) map[string]string {
	t.Helper()
	return pollMenu(t, app, item, menuPath, fmt.Sprintf("no row starting %q", prefix), func(rows map[string]string) bool {
		return !hasLabelPrefix(rows, prefix)
	})
}

// pollMenu reads the menu until ok holds for its rows, and fails the test with what it wanted when time runs out.
func pollMenu(t *testing.T, app *App, item trayItem, menuPath, want string, ok func(map[string]string) bool) map[string]string {
	t.Helper()
	deadline := time.Now().Add(trayWait)
	for {
		rows, err := app.menuRowIDs(item, menuPath)
		must(t, err)
		if ok(rows) {
			return rows
		}
		if time.Now().After(deadline) {
			t.Fatalf("the menu never had %s: %v", want, rows)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func hasLabelPrefix(rows map[string]string, prefix string) bool {
	for label := range rows {
		if strings.HasPrefix(label, prefix) {
			return true
		}
	}
	return false
}

// trayItemWithTitle is a status notifier item and the title it advertises.
type trayItemWithTitle struct {
	trayItem
	title string
}

// trayItemsWithTitles reads the title of every item the watcher lists.
func (a *App) trayItemsWithTitles() ([]trayItemWithTitle, error) {
	entries, err := a.TrayItems()
	if err != nil {
		return nil, err
	}
	var items []trayItemWithTitle
	for _, entry := range entries {
		bus, path, ok := strings.Cut(entry, "/")
		if !ok {
			continue
		}
		item := trayItem{bus: bus, path: "/" + path}
		out, err := a.output("dbus-send", "--session", "--print-reply", "--dest="+item.bus, item.path,
			"org.freedesktop.DBus.Properties.Get", "string:"+statusNotifierItemInterface, "string:Title")
		if err != nil {
			return nil, err
		}
		title := ""
		if values := quotedStrings(out); len(values) > 0 {
			title = values[0]
		}
		items = append(items, trayItemWithTitle{trayItem: item, title: title})
	}
	return items, nil
}

// clickMenuRow sends the dbusmenu clicked event for one row id of the item's menu.
func (a *App) clickMenuRow(item trayItem, menuPath, id string) error {
	var rowID int
	if _, err := fmt.Sscanf(id, "%d", &rowID); err != nil {
		return fmt.Errorf("row id %q: %w", id, err)
	}
	_, err := a.output("dbus-send", "--session", "--print-reply", "--dest="+item.bus, menuPath,
		trayRowAction, fmt.Sprintf("int32:%d", rowID), "string:clicked", "variant:int32:0", "uint32:0")
	return err
}
