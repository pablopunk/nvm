package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// windowRowView is a list whose one row runs action, the way an extension's window opener does.
func windowRowView(action api.Action) api.View {
	return api.View{ID: "ticker-list", Kind: api.ViewList, Title: "Ticker", Items: []api.Item{
		{ID: "open", Title: "Open ticker window", Primary: &action},
	}}
}

// TestWindowRowHidesThePaletteAndHandsTheActionToTheHost runs the row's action and checks the host got the extension's
// window action after the palette hid.
func TestWindowRowHidesThePaletteAndHandsTheActionToTheHost(t *testing.T) {
	var gotExt string
	var gotActions []api.Action
	deps := Deps{Window: func(extID string, action api.Action, identity any) {
		gotExt = extID
		gotActions = append(gotActions, action)
	}}
	window := api.Action{Kind: api.ActionWindow, Op: "toggle", Window: &api.WindowRef{ID: "ticker-floating"}}
	session := NewSession(deps, "ticker", windowRowView(window))
	host := &fakeHost{}
	session.Open(host)
	rows := session.rows("")
	if len(rows) != 1 || len(rows[0].Actions) == 0 {
		t.Fatalf("rows = %+v, want the window row with its action", rows)
	}
	rows[0].Actions[0].Run(host)
	if !host.hidden {
		t.Fatal("the palette stayed open for a window action")
	}
	if gotExt != "ticker" || len(gotActions) != 1 || gotActions[0].Op != "toggle" {
		t.Fatalf("host got extension %q actions %+v, want the toggle of ticker", gotExt, gotActions)
	}
}

// TestWindowActionWithoutAHostWindowsSaysSo keeps a palette without a window host from silently doing nothing.
func TestWindowActionWithoutAHostWindowsSaysSo(t *testing.T) {
	window := api.Action{Kind: api.ActionWindow, Op: "close", Window: &api.WindowRef{ID: "ticker-floating"}}
	session := NewSession(Deps{}, "ticker", windowRowView(window))
	host := &fakeHost{}
	session.Open(host)
	session.rows("")[0].Actions[0].Run(host)
	if len(host.toasts) != 1 || host.toasts[0] != unavailableMessage {
		t.Fatalf("toasts = %v, want the unavailable message", host.toasts)
	}
}

// TestWindowPageDrawsItsRowsInAPalette is an L2 test: the page a companion window shows is drawn by a palette, with the
// view's rows, without a native window.
func TestWindowPageDrawsItsRowsInAPalette(t *testing.T) {
	window := fixtures.ExtensionWindowFixtures()[0]
	session := NewSession(Deps{}, "ticker", window.View)
	page := session.Open(&fakeHost{})
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	tt := ui.NewTester(p.View, 560, 420)
	p.Show()
	tt.Frame()
	if !tt.HasText("ACME 142.10") {
		t.Fatalf("window page does not draw the row %q", "ACME 142.10")
	}
}

func TestWindowActionConfirmationKeepsOriginalInstallation(t *testing.T) {
	identity := new(byte)
	current := identity
	calls := 0
	deps := Deps{CommandSnapshotIdentity: func(string) any { return current }, Window: func(id string, action api.Action, actor any) {
		calls++
		if id != "acme.window" || actor != identity || action.Op != "show" {
			t.Fatal("confirmation changed original window authority")
		}
	}}
	session := NewSession(deps, "acme.window", api.ListView("Window"))
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionWindow, Op: "show", Window: &api.WindowRef{ID: "sample"}, Confirm: &api.Confirmation{}}
	session.action(action, "window").Run(host)
	if calls != 0 || host.hidden || len(host.pushed) != 1 {
		t.Fatal("confirmation skipped")
	}
	current = new(byte)
	confirm := host.pushed[0].Search("")[0].Actions[0]
	confirm.Run(host)
	confirm.Run(host)
	if calls != 1 || !host.hidden {
		t.Fatal("window confirmation was not single use")
	}
}
