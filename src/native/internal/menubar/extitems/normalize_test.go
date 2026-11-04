package extitems

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func rowsOf(n int) []api.MenuRow {
	rows := make([]api.MenuRow, n)
	for i := range rows {
		rows[i] = api.MenuRow{ID: "r" + strings.Repeat("x", i%3), Title: "Row"}
	}
	return rows
}

func TestNormalizeKeepsAWellFormedItemUnchanged(t *testing.T) {
	bars, issues := Normalize([]api.MenuBar{{
		ID: "main", Icon: "timer", Title: "25:00", Tooltip: "Pomodoro",
		Sections: []api.MenuSection{{Title: "Timer", Rows: []api.MenuRow{{ID: "toggle", Title: "Start"}}}},
	}}, "acme.pomodoro")
	if len(issues) != 0 {
		t.Fatalf("issues = %v", issues)
	}
	if len(bars) != 1 || bars[0].Title != "25:00" || bars[0].Icon != "timer" || bars[0].Tooltip != "Pomodoro" {
		t.Fatalf("bars = %+v", bars)
	}
}

func TestNormalizeLimitsItemsAndGivesTheFirstOneTheMainID(t *testing.T) {
	bars, issues := Normalize([]api.MenuBar{{Title: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, "x")
	if len(bars) != 3 || bars[0].ID != "main" {
		t.Fatalf("bars = %+v", bars)
	}
	if !containsIssue(issues, "at most 3 menu bar items") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeDropsExtraItemsWithoutIDsAndDuplicates(t *testing.T) {
	bars, issues := Normalize([]api.MenuBar{{ID: "a"}, {Title: "no id"}, {ID: "a"}}, "x")
	if len(bars) != 1 || bars[0].ID != "a" {
		t.Fatalf("bars = %+v", bars)
	}
	if !containsIssue(issues, "stable id") || !containsIssue(issues, `"a" is used twice`) {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeShortensTitlesAndFallsBackOnUnknownIcons(t *testing.T) {
	bars, issues := Normalize([]api.MenuBar{{
		ID: "main", Title: "this title is far too long for the bar", Icon: "not-a-glyph",
	}}, "x")
	if got := bars[0].Title; len([]rune(got)) != maxBarTitle || !strings.HasSuffix(got, "…") {
		t.Fatalf("title = %q", got)
	}
	if bars[0].Icon != api.MenuIconDefault {
		t.Fatalf("icon = %q, want the default", bars[0].Icon)
	}
	if !containsIssue(issues, "at most 24 characters") || !containsIssue(issues, "unknown menu bar icon") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeCapsSectionsAndRowsAndDropsRowsWithoutIdentity(t *testing.T) {
	sections := make([]api.MenuSection, 10)
	for i := range sections {
		sections[i] = api.MenuSection{Rows: rowsOf(45)}
	}
	sections[0].Rows = append([]api.MenuRow{{ID: "", Title: "no id"}, {ID: "t", Title: ""}}, sections[0].Rows...)
	bars, issues := Normalize([]api.MenuBar{{ID: "main", Sections: sections}}, "x")
	if got := len(bars[0].Sections); got != maxSections {
		t.Fatalf("sections = %d, want %d", got, maxSections)
	}
	if got := len(bars[0].Sections[1].Rows); got != maxRowsPerSection {
		t.Fatalf("rows = %d, want %d", got, maxRowsPerSection)
	}
	if !containsIssue(issues, "at most 8 sections") || !containsIssue(issues, "need an id and a title") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeAllowsOneSubmenuLevelOnly(t *testing.T) {
	nested := api.MenuRow{ID: "a", Title: "A", Items: []api.MenuRow{{ID: "b", Title: "B", Items: []api.MenuRow{{ID: "c", Title: "C"}}}}}
	bars, issues := Normalize([]api.MenuBar{{ID: "main", Sections: []api.MenuSection{{Rows: []api.MenuRow{nested}}}}}, "x")
	row := bars[0].Sections[0].Rows[0]
	if len(row.Items) != 1 || row.Items[0].Items != nil {
		t.Fatalf("submenu depth not capped: %+v", row)
	}
	if !containsIssue(issues, "one level only") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeDropsAnInvalidActionSoItsRowShowsDisabled(t *testing.T) {
	bad := api.Action{Kind: api.ActionRun}
	bars, issues := Normalize([]api.MenuBar{{ID: "main", Sections: []api.MenuSection{{Rows: []api.MenuRow{{ID: "a", Title: "A", Action: &bad}}}}}}, "x")
	if bars[0].Sections[0].Rows[0].Action != nil {
		t.Fatal("an action without a ref survived normalization")
	}
	if !containsIssue(issues, "menu row action is dropped") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestNormalizeTooltipFallsBackToTheExtensionTitle(t *testing.T) {
	bars, _ := Normalize([]api.MenuBar{{ID: "main"}}, "Pomodoro extension")
	if bars[0].Tooltip != "Pomodoro extension" {
		t.Fatalf("tooltip = %q", bars[0].Tooltip)
	}
}

func containsIssue(issues []string, fragment string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, fragment) {
			return true
		}
	}
	return false
}
