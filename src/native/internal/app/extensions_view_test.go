package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionListRows is the Extensions list for ctl as the view answers the query q.
func extensionListRows(ctl extensionController, q string) []search.Item {
	return (&App{}).extensionsViewFor(fixedController(ctl)).Search(q)
}

func TestExtensionStateLabelNamesEachState(t *testing.T) {
	tests := []struct {
		name    string
		listing manager.Listing
		want    string
	}{
		{"running", goodListing(), "Running"},
		{"starting", startingListing(), "Starting"},
		{"restarting after a failure", restartingListing(), "Restarting"},
		{"broken", brokenListing(), "Broken"},
		{"disabled after repeated failures", crashListing(), "Disabled after failures"},
		{"disabled", spinListing(), "Disabled"},
		{"invalid", invalidListing(), "Invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extensionStateLabel(tt.listing); got != tt.want {
				t.Fatalf("state label = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFailedExtensionRowShowsItsReasonInTheSubtitle(t *testing.T) {
	tests := []struct {
		listing manager.Listing
		state   string
		reason  string
	}{
		{brokenListing(), "Broken", brokenListing().Runtime.Reason},
		{crashListing(), "Disabled after failures", crashListing().Runtime.Reason},
		{restartingListing(), "Restarting", restartingListing().Runtime.Reason},
		{invalidListing(), "Invalid", invalidListing().Problems[0]},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			ctl := &fakeExtensions{listings: []manager.Listing{tt.listing}}
			row := findRowTitled(t, extensionListRows(ctl, ""), extensionTitle(tt.listing))
			if row.Accessory != tt.state {
				t.Fatalf("accessory = %q, want %q", row.Accessory, tt.state)
			}
			if !strings.Contains(row.Subtitle, tt.reason) {
				t.Fatalf("subtitle %q does not show the reason %q", row.Subtitle, tt.reason)
			}
		})
	}
}

func TestRunningExtensionRowShowsOnlyItsID(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}}
	row := findRowTitled(t, extensionListRows(ctl, ""), "Good fixture")
	if row.Subtitle != "acme.good" || row.Accessory != "Running" {
		t.Fatalf("row = subtitle %q accessory %q, want the id and Running", row.Subtitle, row.Accessory)
	}
}

func TestEnableOnAnOffExtensionOpensItsReviewBeforeAnyManagerCall(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{spinListing()}}
	row := findRowTitled(t, extensionListRows(ctl, ""), "Hanging fixture")
	h := &viewHost{}
	actionTitled(t, row, "Enable").Run(h)

	if len(ctl.calls) != 0 {
		t.Fatalf("manager calls = %v, want none before the review", ctl.calls)
	}
	view, ok := singlePushed(t, h).(*palette.View)
	if !ok || view.Title != "Review Hanging fixture" {
		t.Fatalf("pushed view = %#v, want the review of the extension", h.pushed[0])
	}
}

func TestEnableIsOfferedOnlyForAnOffExtensionThatLoads(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing(), invalidListing(), spinListing()}}
	rows := extensionListRows(ctl, "")
	if hasAction(findRowTitled(t, rows, "Good fixture"), "Enable") {
		t.Fatal("a running extension offers Enable")
	}
	if hasAction(findRowTitled(t, rows, "acme.bad"), "Enable") {
		t.Fatal("an invalid folder offers Enable, which can only fail")
	}
	if !hasAction(findRowTitled(t, rows, "Hanging fixture"), "Enable") {
		t.Fatal("an off extension that loads does not offer Enable")
	}
}

func TestDisableTurnsTheExtensionOffThroughTheManager(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Good fixture"), "Disable").Run(h)

	if want := []string{"disable acme.good"}; !reflect.DeepEqual(ctl.calls, want) {
		t.Fatalf("manager calls = %v, want %v", ctl.calls, want)
	}
	if h.refresh != 1 {
		t.Fatalf("refreshes = %d, want the list refreshed once", h.refresh)
	}
}

func TestResetAfterRepeatedFailuresEnablesWithoutReview(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{crashListing()}}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Crashing fixture"), "Reset").Run(h)

	if want := []string{"enable acme.crash"}; !reflect.DeepEqual(ctl.calls, want) {
		t.Fatalf("manager calls = %v, want %v", ctl.calls, want)
	}
	if len(h.pushed) != 0 {
		t.Fatalf("reset pushed %d views, want none: the extension is already on", len(h.pushed))
	}
}

func TestDisableFailureIsReportedAsAToast(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}, err: errors.New("save enabled state: disk full")}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Good fixture"), "Disable").Run(h)

	if !reflect.DeepEqual(h.toasts, []string{"save enabled state: disk full"}) {
		t.Fatalf("toasts = %v, want the manager's error", h.toasts)
	}
}

func TestShowSourceOpensTheSourceAsAPane(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}, sources: map[string]string{"acme.good": "package main\n"}}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Good fixture"), "Show Source").Run(h)

	view, ok := singlePushed(t, h).(*palette.View)
	if !ok || view.Title != "Good fixture source" || view.Body == nil || !view.HideInput {
		t.Fatalf("pushed view = %#v, want the source pane titled after the extension", h.pushed[0])
	}
}

func TestShowSourceOfAnUnknownExtensionIsReportedNotPushed(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Good fixture"), "Show Source").Run(h)

	if len(h.pushed) != 0 || len(h.toasts) != 1 {
		t.Fatalf("pushed %d views and toasted %v, want a toast and no view", len(h.pushed), h.toasts)
	}
}

func TestDeleteUninstallsTheExtensionAndRefreshes(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing()}}
	h := &viewHost{}
	actionTitled(t, findRowTitled(t, extensionListRows(ctl, ""), "Good fixture"), "Delete").Run(h)

	if want := []string{"uninstall acme.good"}; !reflect.DeepEqual(ctl.calls, want) {
		t.Fatalf("manager calls = %v, want %v", ctl.calls, want)
	}
	if h.refresh != 1 {
		t.Fatalf("refreshes = %d, want 1", h.refresh)
	}
}

func TestListMatchesTitleOrID(t *testing.T) {
	ctl := &fakeExtensions{listings: []manager.Listing{goodListing(), spinListing()}}
	for _, query := range []string{"hanging", "acme.spin"} {
		rows := extensionListRows(ctl, query)
		if findExtensionRow(rows, "Hanging fixture") == nil || findExtensionRow(rows, "Good fixture") != nil {
			t.Fatalf("query %q lists %v, want only the hanging fixture", query, rowTitlesOf(rows))
		}
	}
}

func TestListWithoutAManagerOffersOnlyTheCommands(t *testing.T) {
	view := (&App{}).extensionsViewFor(func() extensionController { return nil })
	rows := view.Search("")
	if len(rows) != 2 || rows[0].Title != "Create Extension with AI" || rows[1].Title != "Reload Extensions" {
		t.Fatalf("rows = %v, want the create and reload commands only", rowTitlesOf(rows))
	}
	h := &viewHost{}
	actionTitled(t, rows[1], "Reload").Run(h)
	if !reflect.DeepEqual(h.toasts, []string{"Extensions are still loading"}) {
		t.Fatalf("toasts = %v, want the loading message", h.toasts)
	}
}

func TestCapabilityLinesMarkTheUnknownOnes(t *testing.T) {
	got := capabilityLines(reviewListing())
	want := []string{"desktop.apps", "remote.control (unknown)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capability lines = %v, want %v", got, want)
	}
}

func TestNewestLinesKeepTheLastOnes(t *testing.T) {
	got := newestLines([]string{"a", "b", "c", "d"}, 2)
	if !reflect.DeepEqual(got, []string{"c", "d"}) {
		t.Fatalf("newest lines = %v, want the last two", got)
	}
}

func singlePushed(t *testing.T, h *viewHost) any {
	t.Helper()
	if len(h.pushed) != 1 {
		t.Fatalf("pushed %d views, want 1", len(h.pushed))
	}
	return h.pushed[0]
}

func hasAction(row search.Item, title string) bool {
	for _, a := range row.Actions {
		if a.Title == title {
			return true
		}
	}
	return false
}

func findExtensionRow(rows []search.Item, title string) *search.Item {
	for i := range rows {
		if rows[i].Title == title {
			return &rows[i]
		}
	}
	return nil
}

func rowTitlesOf(rows []search.Item) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}
