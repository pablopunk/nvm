package windows

import (
	"context"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func rowTitled(t *testing.T, rows []search.Item, title string) search.Item {
	t.Helper()
	for _, row := range rows {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("no row %q in %v", title, rowTitles(rows))
	return search.Item{}
}

func rowTitles(rows []search.Item) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}

func TestSwitchListsWindowsAndFiltersByTitleOrApp(t *testing.T) {
	d := newDesk(t)
	rows := windowRows(context.Background(), d.deps, "")
	if got := rowTitles(rows); len(got) != 2 {
		t.Fatalf("rows = %v, want both windows", got)
	}
	if got := rowTitles(windowRows(context.Background(), d.deps, "terminal")); len(got) != 1 || got[0] != "zsh" {
		t.Errorf("filter by app name = %v, want zsh", got)
	}
	if got := rowTitles(windowRows(context.Background(), d.deps, "meeting")); len(got) != 1 {
		t.Errorf("filter by title = %v, want the meeting window", got)
	}
}

func TestSwitchRowFocusesAndPlacesFromItsActions(t *testing.T) {
	d := newDesk(t)
	row := windowRows(context.Background(), d.deps, "terminal")[0]
	if row.Actions[0].Title != "Focus Window" {
		t.Fatalf("first action = %q, want Focus Window", row.Actions[0].Title)
	}
	row.Actions[0].Run(&recordingHost{})
	if front := frontmostPID(t, d); front != otherPID {
		t.Errorf("front app = %d after focus, want the terminal %d", front, otherPID)
	}
	var titles []string
	for _, action := range row.Actions[1:] {
		titles = append(titles, action.Title)
	}
	if strings.Join(titles, ",") != "Left Half,Right Half,Maximize,Center,Restore" {
		t.Errorf("layout actions = %v", titles)
	}
}

func frontmostPID(t *testing.T, d desk) int {
	t.Helper()
	app, found, err := d.windows.Frontmost(context.Background())
	if err != nil || !found {
		t.Fatalf("no front app: %v", err)
	}
	return app.PID
}

func TestSettingsRowsShowTheCurrentValues(t *testing.T) {
	d := newDesk(t)
	d.deps.Settings = windowmgmtSettingsWith(t, func(s *geometry.Settings) { s.InnerGap = 12 })
	rows := settingsRows(d.deps, "")
	inner := rowTitled(t, rows, "Inner Gap")
	if inner.Accessory != "12px" {
		t.Errorf("inner gap accessory = %q, want 12px", inner.Accessory)
	}
	if left := rowTitled(t, rows, "Left Gap"); left.Accessory != "Follows Outer Gap" {
		t.Errorf("left gap accessory = %q", left.Accessory)
	}
}

func TestSettingsSearchFiltersByTitle(t *testing.T) {
	rows := settingsRows(newDesk(t).deps, "gap")
	for _, row := range rows {
		if !strings.Contains(strings.ToLower(row.Title), "gap") {
			t.Errorf("row %q does not match gap", row.Title)
		}
	}
	if len(rows) == 0 {
		t.Fatal("no gap rows")
	}
}

func TestFlagRowTogglesAndRedraws(t *testing.T) {
	d := newDesk(t)
	host := &recordingHost{}
	row := rowTitled(t, settingsRows(d.deps, ""), "Gap on Maximize")
	row.Actions[0].Run(host)
	if d.deps.Settings.Current().ApplyGapToMaximize {
		t.Error("gap on maximize still on after toggle")
	}
	if host.refresh != 1 {
		t.Errorf("refresh = %d, want 1", host.refresh)
	}
}

func TestChoiceRowCyclesItsOptions(t *testing.T) {
	d := newDesk(t)
	row := rowTitled(t, settingsRows(d.deps, ""), "Display Order")
	row.Actions[0].Run(&recordingHost{})
	if got := d.deps.Settings.Current().DisplayOrder; got != geometry.DisplayOrderTopToBottom {
		t.Errorf("display order = %q after cycle", got)
	}
}

func TestNumberViewSavesAValidWholeNumberAndPopsBack(t *testing.T) {
	d := newDesk(t)
	def := settingDefByTitle(t, "Outer Gap")
	host := &recordingHost{}
	rows := numberRows(d.deps, def, "12")
	rows[0].Actions[0].Run(host)
	if got := d.deps.Settings.Current().OuterGap; got != 12 {
		t.Errorf("outer gap = %v, want 12", got)
	}
	if host.pops != 1 {
		t.Errorf("pops = %d, want 1", host.pops)
	}
}

func TestNumberViewRefusesOutOfRangeAndNonNumbers(t *testing.T) {
	d := newDesk(t)
	def := settingDefByTitle(t, "Outer Gap")
	for _, text := range []string{"41", "-1", "1.5", "wide"} {
		rows := numberRows(d.deps, def, text)
		if len(rows[0].Actions) != 0 {
			t.Errorf("%q offers a save", text)
		}
		if rows[0].Title != "Use a whole number from 0 to 40." {
			t.Errorf("%q message = %q", text, rows[0].Title)
		}
	}
	if d.deps.Settings.Current().OuterGap != geometry.DefaultSettings().OuterGap {
		t.Error("an invalid value changed the setting")
	}
}

func TestEdgeGapBlankFollowsTheOuterGap(t *testing.T) {
	d := newDesk(t)
	def := settingDefByTitle(t, "Top Gap")
	d.deps.Settings = windowmgmtSettingsWith(t, func(s *geometry.Settings) { v := 20.0; s.OuterGapTop = &v })
	rows := numberRows(d.deps, def, "")
	rows[0].Actions[0].Run(&recordingHost{})
	if d.deps.Settings.Current().OuterGapTop != nil {
		t.Error("blank did not clear the top gap override")
	}
}

func settingDefByTitle(t *testing.T, title string) settingDef {
	t.Helper()
	for _, def := range settingDefs {
		if def.title == title {
			return def
		}
	}
	t.Fatalf("no setting %q", title)
	return settingDef{}
}
