//go:build rig

package rigtest

import "testing"

// menuRowLabels are the rows of the menu bar item, in the order of the 1.x menu.
var menuRowLabels = []string{"Open Nevermind", "Check for Updates…", "Settings…", "Quit Nevermind"}

// TestJourneyTrayMenuRegistersWithMenuRows checks the menu bar item of WP1.12 on the rig. It is not one of PLAN's
// journeys. The test proves the item registers with the session's tray host, and that the menu the host reads over
// D-Bus has the four 1.x rows. Choosing a row is checked by TestJourneyTrayAppItemSettingsRowOpensSettings.
func TestJourneyTrayMenuRegistersWithMenuRows(t *testing.T) {
	app := Launch(t, Options{})

	item, err := app.trayItemOfApp()
	must(t, err)
	if _, err := app.Screenshot("tray-1-registered"); err != nil {
		t.Errorf("desktop screenshot: %v", err)
	}

	menuPath, err := app.trayMenuPath(item)
	must(t, err)
	rows, err := app.menuRowIDs(item, menuPath)
	must(t, err)
	for _, label := range menuRowLabels {
		if _, ok := rows[label]; !ok {
			t.Fatalf("the menu has no %q row: %v", label, rows)
		}
	}
}
