package menubar

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// oneXLabels is the 1.x menu with the update row in place of Check for Updates…, as the labels read.
func oneXLabels(updateLabel string) []string {
	return []string{"Open Nevermind", "-", updateLabel, "Settings…", "-", "Quit Nevermind"}
}

// rowNamed returns the menu row with id, which the tests look up instead of relying on its position.
func rowNamed(t *testing.T, items []platform.MenuItem, id string) platform.MenuItem {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("no %q row in %q", id, labels(items))
	return platform.MenuItem{}
}

func TestEveryUpdateStateShowsOneRowWhereCheckForUpdatesIs(t *testing.T) {
	cases := []struct {
		name     string
		row      *UpdateRow
		want     string
		disabled bool
	}{
		{name: "idle, checking, current, unsupported or check failed", row: nil, want: "Check for Updates…"},
		{name: "downloading", row: &UpdateRow{Label: "Download Nevermind 2.1.0"}, want: "Download Nevermind 2.1.0"},
		{name: "ready to install", row: &UpdateRow{Label: "Install Nevermind 2.1.0"}, want: "Install Nevermind 2.1.0"},
		{name: "installing", row: &UpdateRow{Label: "Installing Nevermind 2.1.0", Disabled: true}, want: "Installing Nevermind 2.1.0", disabled: true},
		{name: "install failed", row: &UpdateRow{Label: "Download Nevermind 2.1.0"}, want: "Download Nevermind 2.1.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := Rows(Actions{}, nil, Menu{GOOS: "darwin", Update: tc.row})
			if got := labels(items); !slices.Equal(got, oneXLabels(tc.want)) {
				t.Fatalf("menu = %q, want %q", got, oneXLabels(tc.want))
			}
			if row := rowNamed(t, items, updateOrCheckID(tc.row)); row.Disabled != tc.disabled {
				t.Fatalf("row disabled = %v, want %v", row.Disabled, tc.disabled)
			}
		})
	}
}

func updateOrCheckID(row *UpdateRow) string {
	if row == nil {
		return "check-updates"
	}
	return "update"
}

func TestUpdateRowClickRunsItsActionAndNilIsEnabledNoOp(t *testing.T) {
	ran := 0
	items := Rows(Actions{}, nil, Menu{GOOS: "darwin", Update: &UpdateRow{Label: "Download Nevermind 2.1.0", Action: func() { ran++ }}})
	row := rowNamed(t, items, "update")
	if row.Disabled || row.Action == nil {
		t.Fatalf("download row = %+v, want enabled with an action", row)
	}
	row.Action()
	if ran != 1 {
		t.Fatalf("action ran %d times, want 1", ran)
	}
	noop := rowNamed(t, Rows(Actions{}, nil, Menu{GOOS: "darwin", Update: &UpdateRow{Label: "Download Nevermind 2.1.0"}}), "update")
	if noop.Disabled {
		t.Fatal("a download row without an action is disabled; 1.x keeps it enabled")
	}
}

func TestUpdateRowsCarryTheDownloadGlyphOnMacOSOnly(t *testing.T) {
	update := &UpdateRow{Label: "Install Nevermind 2.1.0"}
	mac := rowNamed(t, Rows(Actions{}, nil, Menu{GOOS: "darwin", Update: update}), "update")
	if mac.Icon == nil || !mac.Icon.Template || len(mac.Icon.PNG) == 0 {
		t.Fatalf("macOS update icon = %+v, want a template PNG", mac.Icon)
	}
	for _, goos := range []string{"linux", "windows"} {
		if icon := rowNamed(t, Rows(Actions{}, nil, Menu{GOOS: goos, Update: update}), "update").Icon; icon != nil {
			t.Fatalf("%s update row has an icon; 1.x draws row icons on macOS only", goos)
		}
	}
}

func TestCheckForUpdatesHasNoIcon(t *testing.T) {
	check := rowNamed(t, Rows(Actions{}, nil, Menu{GOOS: "darwin"}), "check-updates")
	if check.Icon != nil {
		t.Fatalf("Check for Updates… has an icon %+v; 1.x shows none", check.Icon)
	}
}

func TestPaletteShortcutShowsOnOpenOffLinuxOnly(t *testing.T) {
	cases := []struct {
		goos, shortcut, want string
	}{
		{"darwin", "Command+Space", "Command+Space"},
		{"windows", "Command+Space", "Command+Space"},
		{"linux", "Command+Space", ""},
		{"darwin", "", ""},
	}
	for _, tc := range cases {
		items := Rows(Actions{}, nil, Menu{GOOS: tc.goos, Accelerator: tc.shortcut})
		if got := rowNamed(t, items, "open").Accelerator; got != tc.want {
			t.Fatalf("%s shortcut %q: open row shows %q, want %q", tc.goos, tc.shortcut, got, tc.want)
		}
	}
}
