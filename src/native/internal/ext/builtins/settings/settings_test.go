package settings

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func rowIDs(rows []api.SettingRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// TestPageRowsAreValidAndKeepTheirOrder checks the page validates, lists launch at login only when the OS supports it,
// and places the menu bar icon style after the update channel.
func TestPageRowsAreValidAndKeepTheirOrder(t *testing.T) {
	for _, supported := range []bool{true, false} {
		rows := pageRows(supported)
		if err := (api.SettingsSpec{Rows: rows}).Validate(); err != nil {
			t.Fatalf("supported=%v: page invalid: %v", supported, err)
		}
		ids := rowIDs(rows)
		hasLaunch := false
		for _, id := range ids {
			hasLaunch = hasLaunch || id == SettingLaunchAtLogin
		}
		if hasLaunch != supported {
			t.Fatalf("supported=%v: launch at login listed = %v", supported, hasLaunch)
		}
	}
	ids := rowIDs(pageRows(false))
	channel, style := indexOf(ids, SettingUpdateChannel), indexOf(ids, SettingMenuBarIconStyle)
	if style != channel+1 {
		t.Fatalf("menu bar icon style at %d, update channel at %d; want the style right after the channel", style, channel)
	}
}

func TestMenuBarStylesAreTheSevenOneXStyles(t *testing.T) {
	if len(menuBarStyles) != 7 || menuBarStyles[0].Value != "bolt-filled" {
		t.Fatalf("menu bar styles = %v; want the seven 1.x styles starting with bolt-filled", menuBarStyles)
	}
}

func indexOf(ids []string, id string) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
}
