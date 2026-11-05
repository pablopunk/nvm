package updates

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeUpdates serves the state the page reads.
type fakeUpdates struct {
	api.Updates
	state api.UpdateState
}

func (u fakeUpdates) State() api.UpdateState { return u.state }

// fakeContext exposes only the updates service.
type fakeContext struct {
	api.Context
	updates fakeUpdates
}

func (c fakeContext) Updates() api.Updates { return c.updates }

func invoke(ref string) api.Invocation {
	return api.Invocation{Ref: api.ActionRef{ID: ref}}
}

func openPage(t *testing.T, state api.UpdateState) api.View {
	t.Helper()
	result, err := Extension{}.RunAction(fakeContext{updates: fakeUpdates{state: state}}, invoke(openAction))
	if err != nil || result.View == nil {
		t.Fatalf("open = %+v, %v; want a page", result, err)
	}
	return *result.View
}

func TestPageIsValidAndRefreshesWhileOpen(t *testing.T) {
	view := openPage(t, api.UpdateState{Status: api.UpdateStatusCurrent, Message: "Nevermind is up to date"})
	if view.Kind != api.ViewList || view.Title != "Updates" || view.Placeholder != "Search updates" {
		t.Fatalf("view = %+v, want the Updates list", view)
	}
	if view.Refresh == nil || view.Refresh.Ref == nil || view.Refresh.Ref.ID != refreshAction || view.Refresh.IntervalMs != refreshIntervalMs {
		t.Fatalf("refresh = %+v, want the refresh action every %d ms", view.Refresh, refreshIntervalMs)
	}
	if err := view.Validate(); err != nil {
		t.Fatalf("page invalid: %v", err)
	}
}

func TestRowsAreStatusCheckAndChannelUntilAnUpdateIsDownloaded(t *testing.T) {
	ids := rowIDs(updateRows(api.UpdateState{Status: api.UpdateStatusCurrent, Channel: api.UpdateChannelStable}))
	if want := []string{rowStatus, rowCheck, rowChannel}; !equal(ids, want) {
		t.Fatalf("rows = %v, want %v", ids, want)
	}
	ids = rowIDs(updateRows(api.UpdateState{Status: api.UpdateStatusDownloaded, DownloadedVersion: "2.0.1"}))
	if want := []string{rowStatus, rowRestart, rowCheck, rowChannel}; !equal(ids, want) {
		t.Fatalf("downloaded rows = %v, want the restart after the status", ids)
	}
}

func TestStatusRowShowsTheHostsLineAndFailuresAsContext(t *testing.T) {
	row := statusRow(api.UpdateState{Status: api.UpdateStatusError, Message: "Update check failed: offline", Channel: api.UpdateChannelNightly})
	if row.Title != "Update check failed: offline" || row.Subtitle != "Nightly channel · Checks every 4 hours" || !row.Contextual || !row.Disabled {
		t.Fatalf("status row = %+v", row)
	}
	unsupported := statusRow(api.UpdateState{Status: api.UpdateStatusUnsupported, Message: "Updates are not available in this build yet"})
	if unsupported.Subtitle != "" {
		t.Fatalf("unsupported subtitle = %q, want none", unsupported.Subtitle)
	}
}

func TestChannelRowNamesTheChannelAndSwitchesIt(t *testing.T) {
	row := channelRow(api.UpdateState{Channel: api.UpdateChannelDev})
	if row.Accessories[0].Text != "Dev" || row.Subtitle != "Includes frequent dev builds and nightly prereleases; a final release still wins when newer" {
		t.Fatalf("channel row = %+v", row)
	}
	if action := row.Actions[0]; action.Kind != api.ActionUpdate || action.Op != api.UpdateOpChannel || action.Title != "Switch" {
		t.Fatalf("channel action = %+v, want the channel op titled Switch", action)
	}
}

func TestRestartRowIsProminentAndDismissesAfterRunning(t *testing.T) {
	row := restartRow(api.UpdateState{Status: api.UpdateStatusDownloaded, DownloadedVersion: "2.0.1"})
	if row.Appearance == nil || row.Appearance.Prominence != api.ProminencePrimary || row.Appearance.Foreground != api.ColorNone {
		t.Fatalf("restart appearance = %+v, want the primary prominence on the neutral foreground", row.Appearance)
	}
	if action := row.Actions[0]; action.Op != api.UpdateOpInstall || !action.DismissAfterRun || row.Subtitle != "Nevermind 2.0.1 is installed; restart to run it" {
		t.Fatalf("restart = %+v", row)
	}
}

func TestUnknownActionIsInvalidInput(t *testing.T) {
	_, err := Extension{}.RunAction(fakeContext{}, invoke("nope"))
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
}

func rowIDs(items []api.Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
