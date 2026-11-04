package account

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/search"
)

// recordingHost is the palette host the rows run against. It records what the rows asked the palette to do.
type recordingHost struct {
	pushed  []any
	toasts  []string
	refresh int
}

func (h *recordingHost) Hide() {}
func (h *recordingHost) Pop()  {}
func (h *recordingHost) Push(view any) {
	h.pushed = append(h.pushed, view)
}
func (h *recordingHost) Toast(msg string) {
	h.toasts = append(h.toasts, msg)
}
func (h *recordingHost) Refresh()          { h.refresh++ }
func (h *recordingHost) Query() string     { return "" }
func (h *recordingHost) SetQuery(q string) {}

// titles lists the row titles, in order.
func titles(items []search.Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Title)
	}
	return out
}

// rowTitled finds the row with title, failing the test when it is missing.
func rowTitled(t *testing.T, items []search.Item, title string) search.Item {
	t.Helper()
	for _, item := range items {
		if item.Title == title {
			return item
		}
	}
	t.Fatalf("no row %q in %v", title, titles(items))
	return search.Item{}
}

// connected is a signed-in snapshot on the production backend.
func connected() auth.Snapshot {
	return auth.Snapshot{
		Environment: backend.EnvironmentProduction,
		BaseURL:     "https://api.nvm.fyi",
		Connected:   true,
		Email:       "pat@example.test",
		SignIn:      auth.SignInState{Phase: auth.PhaseApproved},
	}
}

// TestSignedOutRowsFollowTheSignInPhase checks each phase of the device flow shows the rows the view promises.
func TestSignedOutRowsFollowTheSignInPhase(t *testing.T) {
	cases := []struct {
		name  string
		sign  auth.SignInState
		wants []string
	}{
		{"idle offers the log in", auth.SignInState{Phase: auth.PhaseIdle}, []string{"Log in to Nevermind"}},
		{"starting", auth.SignInState{Phase: auth.PhaseStarting}, []string{"Preparing sign-in"}},
		{"pending shows the code", auth.SignInState{Phase: auth.PhasePending, Code: "CODE-1", VerificationURL: "https://x.test"},
			[]string{"Waiting for approval", "Verification URL", "One-time user code", "Open verification page", "Cancel sign-in"}},
		{"browser failed", auth.SignInState{Phase: auth.PhasePending, BrowserOpenFailed: true},
			[]string{"Browser could not be opened", "Verification URL", "One-time user code", "Open verification page", "Cancel sign-in"}},
		{"denied offers a new one", auth.SignInState{Phase: auth.PhaseDenied}, []string{"Sign-in was denied", "Start a new sign-in"}},
		{"expired offers a new one", auth.SignInState{Phase: auth.PhaseExpired}, []string{"Sign-in code expired", "Start a new sign-in"}},
		{"cancelled offers a new one", auth.SignInState{Phase: auth.PhaseCancelled}, []string{"Sign-in cancelled", "Start a new sign-in"}},
		{"failed shows the message", auth.SignInState{Phase: auth.PhaseFailed, Message: "try again"}, []string{"Sign-in could not be started", "Start a new sign-in"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := auth.Snapshot{BaseURL: "https://api.nvm.fyi", SignIn: tc.sign}
			items := signedOutRows(snapshot, Actions{})
			got := titles(items)
			for _, want := range tc.wants {
				rowTitled(t, items, want)
			}
			if len(got) != len(tc.wants) {
				t.Fatalf("rows = %v, want %v", got, tc.wants)
			}
		})
	}
}

// TestRowActionsRunTheirOperation checks each action row calls the operation the app supplied, and leaves the
// palette open.
func TestRowActionsRunTheirOperation(t *testing.T) {
	var started, cancelled, signedOut bool
	actions := Actions{
		StartSignIn:  func(search.Host) { started = true },
		CancelSignIn: func(search.Host) { cancelled = true },
		SignOut:      func(search.Host) { signedOut = true },
	}
	host := &recordingHost{}

	pending := signedOutRows(auth.Snapshot{SignIn: auth.SignInState{Phase: auth.PhasePending}}, actions)
	rowTitled(t, pending, "Cancel sign-in").Actions[0].Run(host)
	if !cancelled {
		t.Fatal("cancel row did not cancel the sign-in")
	}

	signedOutItems := signedOutRows(auth.Snapshot{}, actions)
	action := rowTitled(t, signedOutItems, "Log in to Nevermind").Actions[0]
	if action.Dismiss != search.DismissNever {
		t.Fatal("the log-in row closes the palette; it must keep it open")
	}
	action.Run(host)
	if !started {
		t.Fatal("log-in row did not start the sign-in")
	}

	signedIn := connectedRows(connected(), actions)
	rowTitled(t, signedIn, "Log out of Nevermind").Actions[0].Run(host)
	if !signedOut {
		t.Fatal("log-out row did not sign out")
	}
}

// TestUnwiredActionsLeaveRowsWithoutActions checks a nil operation produces a row that does nothing, rather than a
// row that panics.
func TestUnwiredActionsLeaveRowsWithoutActions(t *testing.T) {
	items := signedOutRows(auth.Snapshot{}, Actions{})
	if len(rowTitled(t, items, "Log in to Nevermind").Actions) != 0 {
		t.Fatal("an unwired log-in row has an action")
	}
}

// TestAccountRowsShowTheBackendAndTheKey checks the backend row always appears, and the key row follows the BYO
// setting.
func TestAccountRowsShowTheBackendAndTheKey(t *testing.T) {
	source := func() auth.Snapshot { return connected() }
	items := rows(source, Actions{SetBYOKey: func(search.Host) {}, ClearBYOKey: func(search.Host) {}})
	rowTitled(t, items, "Backend")
	rowTitled(t, items, "Use your own provider key")

	withKey := connected()
	withKey.BYO = &auth.BYOSetting{Provider: "openai", ModelID: "m-1"}
	items = rows(func() auth.Snapshot { return withKey }, Actions{ClearBYOKey: func(search.Host) {}})
	cleared := rowTitled(t, items, "Clear BYO provider key")
	if cleared.Subtitle != "Using own key for openai (m-1)" {
		t.Fatalf("subtitle = %q, want the provider and model in use", cleared.Subtitle)
	}
}

// TestSpendCapNoticeAppearsWhenThereIsOne checks the notice row is shown when the snapshot carries one.
func TestSpendCapNoticeAppearsWhenThereIsOne(t *testing.T) {
	snapshot := connected()
	snapshot.Notice = &auth.Notice{Title: "Free AI temporarily paused", Message: "paused"}
	items := rows(func() auth.Snapshot { return snapshot }, Actions{})
	notice := rowTitled(t, items, "Free AI temporarily paused")
	if notice.Subtitle != "paused" {
		t.Fatalf("notice subtitle = %q", notice.Subtitle)
	}
}

// TestAccountSummaryShowsPlanAndBalanceWhenReported checks the signed-in summary includes what the backend said.
func TestAccountSummaryShowsPlanAndBalanceWhenReported(t *testing.T) {
	snapshot := connected()
	snapshot.Account = auth.AccountInfo{Plan: "pro", Balance: 1200, HasBalance: true}
	items := connectedRows(snapshot, Actions{})
	if items[0].Subtitle != "Signed in as pat@example.test · Plan pro · 1200 credits" {
		t.Fatalf("summary = %q", items[0].Subtitle)
	}
}

// TestAccountSearchFiltersRowsByTitle checks the page filters as the user types, ignoring case.
func TestAccountSearchFiltersRowsByTitle(t *testing.T) {
	view := View(func() auth.Snapshot { return connected() }, Actions{})
	filtered := view.Search("LOG OUT")
	if got := titles(filtered); len(got) != 1 || got[0] != "Log out of Nevermind" {
		t.Fatalf("filtered = %v, want only the log-out row", got)
	}
}

// TestEnvironmentPageOffersTheTypedOriginAsPreviewOrCustom checks the typed text becomes a choice, and choosing
// it switches to the matching kind.
func TestEnvironmentPageOffersTheTypedOriginAsPreviewOrCustom(t *testing.T) {
	type switchCall struct {
		kind backend.EnvironmentKind
		url  string
	}
	var calls []switchCall
	actions := Actions{SwitchEnvironment: func(_ search.Host, kind backend.EnvironmentKind, rawURL string) {
		calls = append(calls, switchCall{kind, rawURL})
	}}
	view := EnvironmentView(func() auth.Snapshot { return connected() }, actions)

	items := view.Search("https://pr-42.preview.example")
	rowTitled(t, items, "Production").Actions[0].Run(&recordingHost{})
	rowTitled(t, items, "Use preview URL “https://pr-42.preview.example”").Actions[0].Run(&recordingHost{})
	custom := rowTitled(t, items, "Use custom URL “https://pr-42.preview.example”")
	custom.Actions[0].Run(&recordingHost{})

	want := []switchCall{
		{backend.EnvironmentProduction, ""},
		{backend.EnvironmentPRPreview, "https://pr-42.preview.example"},
		{backend.EnvironmentCustom, "https://pr-42.preview.example"},
	}
	if len(calls) != len(want) {
		t.Fatalf("switches = %+v, want %+v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("switch %d = %+v, want %+v", i, calls[i], want[i])
		}
	}
}

// TestDevelopmentBackendIsOfferedOnlyWhenAllowed checks a packaged build does not list the loopback backend.
func TestDevelopmentBackendIsOfferedOnlyWhenAllowed(t *testing.T) {
	packaged := titles(environmentRows(connected(), Actions{}, ""))
	unpackaged := titles(environmentRows(connected(), Actions{Development: true}, ""))
	if len(packaged) != 1 || len(unpackaged) != 2 {
		t.Fatalf("packaged = %v, unpackaged = %v; development must appear only unpackaged", packaged, unpackaged)
	}
}
