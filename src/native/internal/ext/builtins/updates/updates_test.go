package updates

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeUpdates serves the state the page reads.
type fakeUpdates struct {
	api.Updates
	state api.UpdateState
}

func (u fakeUpdates) State() api.UpdateState { return u.state }

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// fakeContext exposes only the updates service and the clock.
type fakeContext struct {
	api.Context
	updates fakeUpdates
}

func (c fakeContext) Updates() api.Updates { return c.updates }

func (fakeContext) Clock() api.Clock { return fixedClock{now: now} }

var now = time.Date(2026, 11, 23, 16, 0, 0, 0, time.UTC)

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

func TestPageOpensOnTheStatusRowAndRefreshesWhileOpen(t *testing.T) {
	view := openPage(t, api.UpdateState{Status: api.UpdateStatusCurrent, Message: "Nevermind is up to date"})
	if view.Kind != api.ViewList || view.Title != "Updates" || view.Placeholder != "Search updates" || view.SelectedItemID != rowStatus {
		t.Fatalf("view = %+v, want the Updates list with the status row selected", view)
	}
	if view.Refresh == nil || view.Refresh.Ref == nil || view.Refresh.Ref.ID != refreshAction || view.Refresh.IntervalMs != refreshIntervalMs {
		t.Fatalf("refresh = %+v, want the refresh action every %d ms", view.Refresh, refreshIntervalMs)
	}
	if ids := rowIDs(view.Items); !equal(ids, []string{rowStatus, rowChannel}) {
		t.Fatalf("rows = %v, want the status row and the channel", ids)
	}
	if err := view.Validate(); err != nil {
		t.Fatalf("page invalid: %v", err)
	}
}

func TestStatusRowChecksAndSaysWhenItLastChecked(t *testing.T) {
	checked := now.Add(-5 * time.Minute).UnixMilli()
	row := statusRow(api.UpdateState{Status: api.UpdateStatusCurrent, Message: "Nevermind is up to date", Channel: api.UpdateChannelDev, CheckedAtMs: checked}, now)
	if row.Title != "Nevermind is up to date" || row.Subtitle != "Checked 5 minutes ago · Dev channel" || row.Busy || row.Disabled {
		t.Fatalf("current row = %+v", row)
	}
	assertCheck(t, row, "Check Again")
	idle := statusRow(api.UpdateState{Status: api.UpdateStatusIdle}, now)
	if idle.Title != "Check for Updates" || idle.Subtitle != "Not checked yet · Stable channel" {
		t.Fatalf("idle row = %+v", idle)
	}
	assertCheck(t, idle, "Check")
}

func TestStatusRowSpinsWithoutAnActionWhileWorkRuns(t *testing.T) {
	for _, state := range []api.UpdateState{
		{Status: api.UpdateStatusChecking, Message: "Checking for updates"},
		{Status: api.UpdateStatusDownloading, Message: "Downloading Nevermind 2.0.1 (42%)", AvailableVersion: "2.0.1"},
		{Status: api.UpdateStatusInstalling, Message: "Restarting to finish updating Nevermind 2.0.1"},
	} {
		row := statusRow(state, now)
		if !row.Busy || len(row.Actions) != 0 || row.Disabled {
			t.Fatalf("%s row = %+v, want a busy row with no action", state.Status, row)
		}
	}
	checking := statusRow(api.UpdateState{Status: api.UpdateStatusChecking}, now)
	if checking.Title != "Checking for updates…" {
		t.Fatalf("checking title = %q", checking.Title)
	}
	downloading := statusRow(api.UpdateState{Status: api.UpdateStatusDownloading, Message: "Downloading Nevermind 2.0.1 (42%)"}, now)
	if downloading.Title != "Downloading Nevermind 2.0.1 (42%)" {
		t.Fatalf("downloading title = %q, want the updater's progress line", downloading.Title)
	}
}

func TestStatusRowRestartsIntoADownloadedUpdate(t *testing.T) {
	row := statusRow(api.UpdateState{Status: api.UpdateStatusDownloaded, DownloadedVersion: "2.0.1"}, now)
	if row.ID != rowStatus || row.Title != "Restart to Update" || row.Subtitle != "Nevermind 2.0.1 is ready" {
		t.Fatalf("downloaded row = %+v", row)
	}
	if row.Appearance == nil || row.Appearance.Prominence != api.ProminencePrimary {
		t.Fatalf("restart appearance = %+v, want the primary prominence", row.Appearance)
	}
	if action := row.Actions[0]; action.Op != api.UpdateOpInstall || !action.DismissAfterRun {
		t.Fatalf("restart action = %+v", action)
	}
}

func TestStatusRowNamesTheFailedStepAndTriesAgain(t *testing.T) {
	row := statusRow(api.UpdateState{Status: api.UpdateStatusError, FailedStage: api.UpdateStageCheck, ErrorMessage: "no network connection"}, now)
	if row.Title != "Could not check for updates" || row.Subtitle != "no network connection" || row.Icon != "circle-alert" {
		t.Fatalf("failed check row = %+v", row)
	}
	assertCheck(t, row, "Try Again")
	install := statusRow(api.UpdateState{Status: api.UpdateStatusError, FailedStage: api.UpdateStageInstall, ErrorMessage: "bad signature"}, now)
	if install.Title != "Could not install the update" {
		t.Fatalf("failed install title = %q", install.Title)
	}
	unsupported := statusRow(api.UpdateState{Status: api.UpdateStatusUnsupported, Message: "Updates are not available in this build yet"}, now)
	if !unsupported.Disabled || unsupported.Subtitle != "" || len(unsupported.Actions) != 0 {
		t.Fatalf("unsupported row = %+v, want a passive line", unsupported)
	}
}

func TestChannelRowOpensTheChannelList(t *testing.T) {
	row := channelRow(api.UpdateState{Channel: api.UpdateChannelDev})
	if row.Subtitle != "" || row.Accessories[0].Text != "Dev" {
		t.Fatalf("channel row = %+v, want no subtitle and the channel's name", row)
	}
	action := row.Actions[0]
	if action.Kind != api.ActionPushView || action.View == nil || action.View.SelectedItemID != "channel-dev" {
		t.Fatalf("channel action = %+v, want the channel list with Dev selected", action)
	}
	if err := action.View.Validate(); err != nil {
		t.Fatalf("channel list invalid: %v", err)
	}
	for _, choice := range action.View.Items {
		pick := choice.Actions[0]
		if pick.Kind != api.ActionUpdate || pick.Op != api.UpdateOpChannel || "channel-"+pick.Target != choice.ID {
			t.Fatalf("choice %s action = %+v, want the channel op for that channel", choice.ID, pick)
		}
		if marked := len(choice.Accessories) == 1; marked != (choice.ID == "channel-dev") {
			t.Fatalf("choice %s marked = %v, want only the followed channel marked", choice.ID, marked)
		}
	}
	if unsupported := channelRow(api.UpdateState{Status: api.UpdateStatusUnsupported}); !unsupported.Disabled || len(unsupported.Actions) != 0 {
		t.Fatalf("unsupported channel row = %+v, want it passive", unsupported)
	}
}

func TestRootShowsADownloadAsALiveRow(t *testing.T) {
	rows := rootRows(api.UpdateState{Status: api.UpdateStatusDownloading, Message: "Downloading Nevermind 2.0.1 (42%)"})
	if len(rows) != 1 || rows[0].Title != "Downloading Nevermind 2.0.1 (42%)" || !rows[0].Busy || rows[0].Score != rootUpdateScore {
		t.Fatalf("root rows = %+v, want one busy row with the progress line", rows)
	}
	if rows[0].Actions[0].Kind != api.ActionRun || rows[0].Actions[0].Ref.ID != openAction {
		t.Fatalf("downloading action = %+v, want the Updates page", rows[0].Actions[0])
	}
	ready := rootRows(api.UpdateState{Status: api.UpdateStatusDownloaded, Message: "Nevermind 2.0.1 is ready. Restart to finish updating."})
	if ready[0].Busy || ready[0].Actions[0].Op != api.UpdateOpInstall || ready[0].Appearance.Background != "accent" {
		t.Fatalf("ready row = %+v, want the restart", ready[0])
	}
}

func TestRootStaysQuietWithoutAnUpdate(t *testing.T) {
	for _, state := range []api.UpdateState{
		{Status: api.UpdateStatusIdle}, {Status: api.UpdateStatusCurrent}, {Status: api.UpdateStatusChecking},
		{Status: api.UpdateStatusError, FailedStage: api.UpdateStageCheck},
	} {
		if rows := rootRows(state); len(rows) != 0 {
			t.Fatalf("%+v root rows = %+v, want none", state, rows)
		}
	}
	if rows := rootRows(api.UpdateState{Status: api.UpdateStatusError, FailedStage: api.UpdateStageInstall}); len(rows) != 1 {
		t.Fatalf("failed install root rows = %+v, want the failure shown", rows)
	}
}

func TestSearchKeepsTheRowOnlyForQueriesThatNameUpdates(t *testing.T) {
	ctx := fakeContext{updates: fakeUpdates{state: api.UpdateState{Status: api.UpdateStatusDownloading}}}
	if rows, _ := (Extension{}).SearchItems(ctx, "upd"); len(rows) != 1 {
		t.Fatalf("upd rows = %+v, want the update row", rows)
	}
	if rows, _ := (Extension{}).SearchItems(ctx, "clip"); len(rows) != 0 {
		t.Fatalf("clip rows = %+v, want none", rows)
	}
}

func TestTimeAgoReadsAsASentence(t *testing.T) {
	for _, tt := range []struct {
		ago  time.Duration
		want string
	}{{10 * time.Second, "just now"}, {time.Minute, "1 minute ago"}, {2 * time.Hour, "2 hours ago"}, {72 * time.Hour, "3 days ago"}} {
		if got := timeAgo(now, now.Add(-tt.ago)); got != tt.want {
			t.Fatalf("timeAgo(%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestUnknownActionIsInvalidInput(t *testing.T) {
	_, err := Extension{}.RunAction(fakeContext{}, invoke("nope"))
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want invalid input", err)
	}
}

func assertCheck(t *testing.T, row api.Item, title string) {
	t.Helper()
	if len(row.Actions) != 1 || row.Actions[0].Op != api.UpdateOpCheck || row.Actions[0].Title != title {
		t.Fatalf("%s actions = %+v, want one check titled %q", row.ID, row.Actions, title)
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
