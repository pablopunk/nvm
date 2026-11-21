package apps

import (
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var reviewed = api.UninstallReview{
	Token: "uninstall-1", Ready: true,
	Notes: []string{"Some data was skipped."},
	Items: []api.UninstallItem{
		{ID: "app", Path: "/Applications/Zed.app", Kind: "app", Reason: "The application", Selected: true},
		{ID: "prefs", Path: "~/Library/Preferences/dev.zed.plist", Kind: "associated", Reason: "Preferences"},
	},
}

// startReview opens the review of Zed and returns the view it showed.
func startReview(t *testing.T, ext Extension, apps *fakeApps) api.View {
	t.Helper()
	result, err := ext.RunAction(newTestContext(apps), api.Invocation{Ref: api.ActionRef{ID: refUninstall, Args: map[string]string{argPath: "/Applications/Zed.app", argName: "Zed"}}})
	if err != nil || result.View == nil {
		t.Fatalf("uninstall = %+v, %v; want the review", result, err)
	}
	return *result.View
}

// TestUninstallShowsTheReviewWithItsDefaultSelection: the review lists the candidates, the app is selected by default,
// and the notes follow the items.
func TestUninstallShowsTheReviewWithItsDefaultSelection(t *testing.T) {
	view := startReview(t, New(), &fakeApps{review: reviewed})
	if view.Title != "Uninstall Zed" || view.Placeholder != "Filter items to move to Trash" {
		t.Fatalf("view = %q / %q", view.Title, view.Placeholder)
	}
	titles := titlesOf(view.Items)
	if want := []string{"Review selected items", "/Applications/Zed.app", "~/Library/Preferences/dev.zed.plist", "Some data was skipped."}; !slices.Equal(titles, want) {
		t.Fatalf("items = %v, want %v", titles, want)
	}
	if got := view.Items[1].Accessories[0].Text; got != "Selected" {
		t.Fatalf("app accessory = %q, want Selected", got)
	}
	if got := view.Items[2].Accessories[0].Text; got != "Optional" {
		t.Fatalf("data accessory = %q, want Optional", got)
	}
}

// TestUnavailableReviewSaysWhy: a review the host cannot make shows its reason and no toggles.
func TestUnavailableReviewSaysWhy(t *testing.T) {
	view := startReview(t, New(), &fakeApps{review: api.UninstallReview{Message: "This app is in /System."}})
	if view.Title != "Uninstall unavailable" || len(view.Items) != 1 || view.Items[0].Subtitle != "This app is in /System." {
		t.Fatalf("view = %+v", view)
	}
}

// TestToggleFlipsOneCandidateAndPatchesTheList: a toggle changes the accessory of one item, and the review row asks.
func TestToggleFlipsOneCandidateAndPatchesTheList(t *testing.T) {
	apps := &fakeApps{review: reviewed}
	ext := New()
	startReview(t, ext, apps)
	result, err := ext.RunAction(newTestContext(apps), api.Invocation{Ref: api.ActionRef{ID: refToggle, Args: map[string]string{argToken: "uninstall-1", argID: "prefs"}}})
	if err != nil || result.Patch == nil || result.Patch.Mode != api.PatchReplace {
		t.Fatalf("toggle = %+v, %v; want a replace patch", result, err)
	}
	if got := result.Patch.Items[2].Accessories[0].Text; got != "Selected" {
		t.Fatalf("toggled data accessory = %q, want Selected", got)
	}
}

// TestReviewWithNothingSelectedMovesNothing: confirming an empty selection says so and asks nothing.
func TestReviewWithNothingSelectedMovesNothing(t *testing.T) {
	apps := &fakeApps{review: reviewed}
	ext := New()
	startReview(t, ext, apps)
	ctx := newTestContext(apps)
	for _, id := range []string{"app"} {
		if _, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refToggle, Args: map[string]string{argToken: "uninstall-1", argID: id}}}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refReview, Args: map[string]string{argToken: "uninstall-1"}}})
	if err != nil || result.Toast == nil || result.Toast.Message != "Select at least one item to move to Trash" || result.Next != nil {
		t.Fatalf("review = %+v, %v; want the nothing-selected message", result, err)
	}
}

// TestConfirmedTrashMovesTheSelectionAndShowsTheResult: the confirmation names the selected paths, the Trash moves the
// selected IDs under the review's token, and a partial run lists what stayed.
func TestConfirmedTrashMovesTheSelectionAndShowsTheResult(t *testing.T) {
	apps := &fakeApps{review: reviewed, result: api.UninstallResult{
		Status: "partial", TrashFolder: "/Users/t/.Trash", Moved: []string{"/Applications/Zed.app"},
		Untouched: []api.UninstallUntouched{{Path: "~/Library/Preferences/dev.zed.plist", Message: "Quit the app first."}},
	}}
	ext := New()
	startReview(t, ext, apps)
	ctx := newTestContext(apps)
	review, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refReview, Args: map[string]string{argToken: "uninstall-1"}}})
	if err != nil || review.Next == nil || review.Next.Confirm == nil {
		t.Fatalf("review = %+v, %v; want a confirmation", review, err)
	}
	confirm := review.Next.Confirm
	if confirm.Title != "Move 1 item to Trash?" || confirm.ConfirmLabel != "Move 1 item to Trash" || confirm.Message != "/Applications/Zed.app\nIf running, the application will be asked to quit first." {
		t.Fatalf("confirmation = %+v", confirm)
	}
	result, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refTrash, Args: map[string]string{argToken: "uninstall-1"}}})
	if err != nil || apps.trashToken != "uninstall-1" || !slices.Equal(apps.trashed, []string{"app"}) {
		t.Fatalf("trash = %+v, %v; token %q ids %v", result, err, apps.trashToken, apps.trashed)
	}
	if result.View == nil || result.View.Title != "Uninstall partially completed" {
		t.Fatalf("result = %+v, want the partial result", result)
	}
	titles := titlesOf(result.View.Items)
	if !slices.Contains(titles, "Some selected items were left untouched.") || !slices.Contains(titles, "Reveal in Trash") {
		t.Fatalf("result items = %v", titles)
	}
	reveal := result.View.Items[len(result.View.Items)-1].Primary
	if reveal == nil || reveal.Kind != api.ActionRevealPath || reveal.Path != "/Users/t/.Trash" {
		t.Fatalf("reveal = %+v, want the Trash folder", reveal)
	}
}

// TestCompleteTrashSaysHowManyMoved: a complete run is one line.
func TestCompleteTrashSaysHowManyMoved(t *testing.T) {
	view := resultView(newTestContext(&fakeApps{}), api.UninstallResult{Status: "complete", Moved: []string{"a", "b"}})
	if view.Title != "Uninstall" || view.Items[0].Title != "2 items moved to Trash" {
		t.Fatalf("view = %q, first row %q", view.Title, view.Items[0].Title)
	}
	if !strings.Contains(view.Items[len(view.Items)-1].Title, "Reveal in Trash") {
		t.Fatalf("last row = %q, want Reveal in Trash", view.Items[len(view.Items)-1].Title)
	}
}

// TestReviewsAreDroppedOnceTrashed: a review that moved its items cannot be used again.
func TestReviewsAreDroppedOnceTrashed(t *testing.T) {
	apps := &fakeApps{review: reviewed, result: api.UninstallResult{Status: "complete", Moved: []string{"app"}}}
	ext := New()
	startReview(t, ext, apps)
	ctx := newTestContext(apps)
	if _, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refTrash, Args: map[string]string{argToken: "uninstall-1"}}}); err != nil {
		t.Fatal(err)
	}
	result, _ := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refToggle, Args: map[string]string{argToken: "uninstall-1", argID: "app"}}})
	if result.Toast == nil || result.Toast.Message != "This uninstall review has expired" {
		t.Fatalf("toggle after trash = %+v, want the expired message", result)
	}
}
