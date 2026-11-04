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

// reviewRowsFor is the review of l as the review view answers its search.
func reviewRowsFor(ctl extensionController, l manager.Listing) []search.Item {
	return (&App{}).extensionReviewView(ctl, l).Search("")
}

func TestReviewStartsWithEnableThenCancel(t *testing.T) {
	rows := reviewRowsFor(&fakeExtensions{}, reviewListing())
	if len(rows) < 2 || rows[0].Title != "Enable" || rows[1].Title != "Cancel" {
		t.Fatalf("review rows = %v, want Enable then Cancel first", rowTitlesOf(rows))
	}
	if rows[0].Section != "acme.review" {
		t.Fatalf("enable section = %q, want the extension id", rows[0].Section)
	}
}

func TestReviewShowsKnownAndUnknownCapabilities(t *testing.T) {
	rows := reviewRowsFor(&fakeExtensions{}, reviewListing())

	known := findRowTitled(t, rows, "desktop.apps")
	if known.Subtitle != "Declared capability" || known.Accessory != "" || known.Section != "Declared capabilities" {
		t.Fatalf("known capability row = %+v, want a declared capability under its section", known)
	}
	unknown := findRowTitled(t, rows, "remote.control")
	if unknown.Accessory != "Unknown" || !strings.Contains(unknown.Subtitle, "Unknown") {
		t.Fatalf("unknown capability row = %+v, want it marked unknown", unknown)
	}
}

func TestReviewWarnsThatCapabilitiesAreNotASandbox(t *testing.T) {
	rows := reviewRowsFor(&fakeExtensions{}, reviewListing())
	warning := findRowTitled(t, rows, "Capabilities are review metadata, not a sandbox")
	if !strings.Contains(warning.Subtitle, "docs/extension-trust-model.md") {
		t.Fatalf("warning subtitle %q does not point at the trust model", warning.Subtitle)
	}
}

func TestReviewWithoutCapabilitiesSaysSo(t *testing.T) {
	rows := reviewRowsFor(&fakeExtensions{}, spinListing())
	findRowTitled(t, rows, "None declared")
}

func TestReviewEnableCallsTheManagerAndReturns(t *testing.T) {
	ctl := &fakeExtensions{}
	h := &viewHost{}
	actionTitled(t, reviewRowsFor(ctl, reviewListing())[0], "Enable").Run(h)

	if want := []string{"enable acme.review"}; !reflect.DeepEqual(ctl.calls, want) {
		t.Fatalf("manager calls = %v, want %v", ctl.calls, want)
	}
	if h.pops != 1 || len(h.toasts) != 0 {
		t.Fatalf("pops = %d toasts = %v, want one return and no message", h.pops, h.toasts)
	}
}

func TestReviewEnableFailureReturnsAndReportsIt(t *testing.T) {
	ctl := &fakeExtensions{err: errors.New("extension acme.review does not load: broken")}
	h := &viewHost{}
	actionTitled(t, reviewRowsFor(ctl, reviewListing())[0], "Enable").Run(h)

	if h.pops != 1 {
		t.Fatalf("pops = %d, want the review to return to the list", h.pops)
	}
	if !reflect.DeepEqual(h.toasts, []string{ctl.err.Error()}) {
		t.Fatalf("toasts = %v, want the manager's error", h.toasts)
	}
}

func TestReviewCancelReturnsWithoutEnabling(t *testing.T) {
	ctl := &fakeExtensions{}
	h := &viewHost{}
	actionTitled(t, reviewRowsFor(ctl, reviewListing())[1], "Cancel").Run(h)

	if len(ctl.calls) != 0 {
		t.Fatalf("manager calls = %v, want none", ctl.calls)
	}
	if h.pops != 1 {
		t.Fatalf("pops = %d, want 1", h.pops)
	}
}

func TestReviewViewIsTitledAfterTheExtension(t *testing.T) {
	view := (&App{}).extensionReviewView(&fakeExtensions{}, reviewListing())
	if view.Title != "Review Capability fixture" || view.Presentation != palette.Stacked {
		t.Fatalf("review view title = %q, want it named after the extension", view.Title)
	}
}
