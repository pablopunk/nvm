package apps

import (
	"reflect"
	"strings"
	"testing"
)

// View tests: the apps rows, the Force Quit and Quit All views, the uninstall review, the confirmation and the result,
// as plain data. Ported cases from apps.test.ts keep the 1.x wording and order; they name their source below.

func displayHome(p string) string { return DisplayPath(p, testHome) }

// Ported from apps.test.ts: "app root/search work stays synchronous while Uninstall is a macOS-only lazy secondary
// action". The row exists without any discovery; Uninstall is an action, after Open, only where it is supported.
func TestAppRowsKeepOpenPrimaryAndOfferUninstallLazily(t *testing.T) {
	list := []App{{Name: "Example", Path: "/Applications/Example.app", FirstSeenAt: 1234}}
	rows := AppRows(list, map[string]bool{}, true)
	row := rows[0]
	if row.Actions[0].Kind != ActionOpen || row.Actions[0].Title != "Open Example" {
		t.Fatalf("primary = %+v", row.Actions[0])
	}
	if last := row.Actions[len(row.Actions)-1]; last.Kind != ActionUninstall || last.Title != "Uninstall Example…" {
		t.Fatalf("uninstall action = %+v", last)
	}
	for _, a := range AppRows(list, map[string]bool{}, false)[0].Actions {
		if a.Kind == ActionUninstall {
			t.Fatal("uninstall offered where it is unsupported")
		}
	}
}

// A running app shows the running state as its subtitle and offers Quit and Force Quit; an idle app shows neither.
func TestAppRowsShowRunningStateAndQuitActions(t *testing.T) {
	list := []App{{Name: "Notes", Path: "/Applications/Notes.app"}, {Name: "Mail", Path: "/Applications/Mail.app"}}
	rows := AppRows(list, map[string]bool{"/Applications/Notes.app": true}, false)
	if rows[0].Subtitle != RunningSubtitle || !rows[0].Running {
		t.Fatalf("running row = %+v", rows[0])
	}
	var kinds []ActionKind
	for _, a := range rows[0].Actions {
		kinds = append(kinds, a.Kind)
	}
	if want := []ActionKind{ActionOpen, ActionReveal, ActionCopyPath, ActionQuit, ActionForceQuit}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("running actions = %v, want %v", kinds, want)
	}
	if rows[1].Subtitle != "" || rows[1].Running {
		t.Fatalf("idle row = %+v", rows[1])
	}
	force := rows[0].Actions[4]
	if !force.Destructive || force.Confirm != "Force Quit" || force.ConfirmMessage != "Force quit Notes? Unsaved data may be lost." {
		t.Fatalf("force quit = %+v", force)
	}
}

// The Force Quit Apps view lists only the running apps. Each row's only action is the force quit, which Enter runs,
// and it carries the 1.x subtitle.
func TestForceQuitRowsListOnlyRunningAppsAndForceQuitFirst(t *testing.T) {
	list := []App{{Name: "Notes", Path: "/Applications/Notes.app"}, {Name: "Mail", Path: "/Applications/Mail.app"}}
	rows := ForceQuitRows(list, map[string]bool{"/Applications/Mail.app": true})
	if len(rows) != 1 || rows[0].Title != "Mail" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Subtitle != "Force quit this application" {
		t.Fatalf("subtitle = %q", rows[0].Subtitle)
	}
	if len(rows[0].Actions) != 1 || rows[0].Actions[0].Kind != ActionForceQuit || rows[0].Actions[0].Confirm != "Force Quit" {
		t.Fatalf("actions = %+v", rows[0].Actions)
	}
	if empty := ForceQuitRows(list, map[string]bool{}); len(empty) != 0 {
		t.Fatalf("rows with nothing running = %+v", empty)
	}
}

// Quit All names the apps it will ask to close, leaves Nevermind out, and lists the exceptions it skips.
func TestQuitAllPlanSplitsQuitAndExceptions(t *testing.T) {
	self := App{Name: "Nevermind", Path: "/Applications/Nevermind.app"}
	list := []App{
		{Name: "Notes", Path: "/Applications/Notes.app"},
		{Name: "Finder", Path: "/System/Library/CoreServices/Finder.app"},
		self,
		{Name: "Idle", Path: "/Applications/Idle.app"},
	}
	running := map[string]bool{
		"/Applications/Notes.app":                 true,
		"/System/Library/CoreServices/Finder.app": true,
		"/Applications/Nevermind.app":             true,
	}
	quit, excepted := QuitAllPlan(list, running, self)
	if len(quit) != 1 || quit[0].Name != "Notes" {
		t.Fatalf("quit = %+v", quit)
	}
	if len(excepted) != 1 || excepted[0].Name != "Finder" {
		t.Fatalf("excepted = %+v", excepted)
	}
}

// Ported from apps.test.ts: "Uninstall candidate list defaults to the app, patches selection, and confirms host
// snapshot paths". The confirmation text is the 1.x text, line for line.
func TestUninstallReviewAndConfirmationMatchTheOneXWording(t *testing.T) {
	d := Discovery{Ready: true, Snapshot: UninstallSnapshot{AppPath: testApp, Candidates: []Candidate{
		{ID: "app-id", Path: testApp, Kind: KindApp, SelectedByDefault: true, Reason: "Application bundle"},
		{ID: "cache-id", Path: testCache, Kind: KindAssociated, Reason: "Bundle ID prefix match; may belong to another app"},
	}}, Notes: []Note{{Code: "missing-associated", Message: "Some locations missing"}}}
	selected := map[string]bool{"app-id": true}
	review := BuildReview(d, selected, displayHome)
	if review.Title != "Uninstall Example" {
		t.Fatalf("title = %q", review.Title)
	}
	if review.Items[0].Selected != true || review.Items[1].Selected != false || review.Items[1].Title != "~/Library/Caches/com.example.App" {
		t.Fatalf("items = %+v", review.Items)
	}
	if !reflect.DeepEqual(review.Notes, []string{"Some locations missing"}) {
		t.Fatalf("notes = %v", review.Notes)
	}

	selected["cache-id"] = true
	title, message := ConfirmText(Selected(d.Snapshot, selected), displayHome)
	if title != "Move 2 items to Trash" {
		t.Fatalf("title = %q", title)
	}
	want := testApp + "\n~/Library/Caches/com.example.App\nIf running, the application will be asked to quit first."
	if message != want {
		t.Fatalf("message = %q, want %q", message, want)
	}
	if NoSelectionMessage != "Select at least one item to move to Trash" {
		t.Fatalf("no-selection message = %q", NoSelectionMessage)
	}
}

// A single item uses the singular noun, and an app-less selection has no quit line.
func TestConfirmTextSingularAndWithoutAppLine(t *testing.T) {
	title, message := ConfirmText([]Candidate{{Path: testCache, Kind: KindAssociated}}, displayHome)
	if title != "Move 1 item to Trash" || strings.Contains(message, "quit") {
		t.Fatalf("title %q, message %q", title, message)
	}
}

// Ported from apps.test.ts: the complete run is the one-line success; the partial run lists what stayed and what moved.
func TestResultViewsForCompleteAndPartialRuns(t *testing.T) {
	complete := BuildResult(TrashResult{Status: StatusComplete, Moved: []string{testApp}}, displayHome)
	if !complete.Success || complete.Summary != "1 item moved to Trash" {
		t.Fatalf("complete = %+v", complete)
	}
	partial := BuildResult(TrashResult{
		Status:    StatusPartial,
		Moved:     []string{testApp},
		Untouched: []Untouched{{Path: testCache, Refusal: Refusal{Code: codeTrashFailed, Message: "Not moved"}}},
	}, displayHome)
	if partial.Title != "Uninstall partially completed" || partial.Success {
		t.Fatalf("partial = %+v", partial)
	}
	var details []string
	for _, d := range partial.Details {
		details = append(details, d.Title+" | "+d.Subtitle)
	}
	want := []string{"~/Library/Caches/com.example.App | Not moved", testApp + " | Moved to Trash"}
	if !reflect.DeepEqual(details, want) {
		t.Fatalf("details = %v, want %v", details, want)
	}
	failed := BuildResult(TrashResult{Status: StatusFailed}, displayHome)
	if failed.Title != "Nothing was moved to Trash" {
		t.Fatalf("failed title = %q", failed.Title)
	}
}

// DisplayPath shortens the home folder only at a path boundary.
func TestDisplayPathShortensOnlyAtPathBoundaries(t *testing.T) {
	cases := map[string]string{
		"/Users/tester":          "~",
		"/Users/tester/Library":  "~/Library",
		"/Users/testerx/Library": "/Users/testerx/Library",
		"/Applications/x.app":    "/Applications/x.app",
	}
	for in, want := range cases {
		if got := DisplayPath(in, testHome); got != want {
			t.Errorf("DisplayPath(%q) = %q, want %q", in, got, want)
		}
	}
}
