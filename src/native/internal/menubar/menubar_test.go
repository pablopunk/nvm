package menubar

import (
	"bytes"
	"errors"
	"image/png"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// recorder counts what each menu action was asked to do, in the order the actions ran.
type recorder struct {
	calls []string
}

func (r *recorder) actions() Actions {
	return Actions{
		Open:            func() { r.calls = append(r.calls, "open") },
		CheckForUpdates: func() { r.calls = append(r.calls, "check-updates") },
		Settings:        func() { r.calls = append(r.calls, "settings") },
		Quit:            func() { r.calls = append(r.calls, "quit") },
	}
}

// labels lists the menu rows' labels, with dividers shown as "-".
func labels(items []platform.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Separator {
			out = append(out, "-")
			continue
		}
		out = append(out, item.Label)
	}
	return out
}

func TestMenuKeepsTheOneXOrder(t *testing.T) {
	want := []string{"Open Nevermind", "-", "Check for Updates…", "Settings…", "-", "Quit Nevermind"}
	got := labels(Rows(Actions{}, nil))
	if !slices.Equal(got, want) {
		t.Fatalf("menu = %q, want %q", got, want)
	}
}

func TestNewCreatesTheItemWithIconTooltipAndMenu(t *testing.T) {
	host := fake.NewTrayHost()
	rec := &recorder{}
	item, err := New(Options{Host: host, Actions: rec.actions()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trays := host.Trays()
	if len(trays) != 1 {
		t.Fatalf("items = %d, want 1", len(trays))
	}
	opts := trays[0].Options()
	if opts.ToolTip != Tooltip || opts.ID != "nevermind" {
		t.Fatalf("options = %+v", opts)
	}
	if _, err := png.Decode(bytes.NewReader(opts.Icon)); err != nil {
		t.Fatalf("icon is not PNG: %v", err)
	}
	if got := labels(trays[0].Items()); len(got) != 6 {
		t.Fatalf("menu rows = %q", got)
	}
	item.Close()
	if !trays[0].Closed() {
		t.Fatal("Close did not remove the item")
	}
}

func TestEachRowRunsItsActionOnceWhenInvoked(t *testing.T) {
	host := fake.NewTrayHost()
	rec := &recorder{}
	if _, err := New(Options{Host: host, Actions: rec.actions()}); err != nil {
		t.Fatalf("New: %v", err)
	}
	tray := host.Trays()[0]
	for _, id := range []string{"open", "check-updates", "settings", "quit"} {
		if err := tray.Invoke(id); err != nil {
			t.Fatalf("Invoke(%s): %v", id, err)
		}
	}
	want := []string{"open", "check-updates", "settings", "quit"}
	if !slices.Equal(rec.calls, want) {
		t.Fatalf("calls = %q, want %q", rec.calls, want)
	}
}

func TestRowsRunThroughTheUIThreadDispatcher(t *testing.T) {
	var dispatched []string
	onUI := func(fn func()) {
		dispatched = append(dispatched, "ui")
		fn()
	}
	rec := &recorder{}
	rows := Rows(rec.actions(), onUI)
	for _, row := range rows {
		if row.Separator {
			continue
		}
		row.Action()
	}
	if len(dispatched) != 4 || len(rec.calls) != 4 {
		t.Fatalf("dispatched %d, ran %d; want 4 each", len(dispatched), len(rec.calls))
	}
}

func TestNilActionsLeaveRowsWithoutAnAction(t *testing.T) {
	for _, row := range Rows(Actions{}, func(fn func()) { fn() }) {
		if !row.Separator && row.Action != nil {
			t.Fatalf("row %q has an action without one being set", row.Label)
		}
	}
}

func TestHostFailureIsReturnedAndNoItemIsKept(t *testing.T) {
	host := fake.NewTrayHost()
	host.FailNext(fake.OpTrayNewTray, errors.New("no tray host"))
	item, err := New(Options{Host: host})
	if err == nil {
		t.Fatal("New succeeded without a tray host")
	}
	if item != nil {
		t.Fatal("New returned an item with the error")
	}
}

func TestTitleTracksTheTimerAndNilItemIsSafe(t *testing.T) {
	host := fake.NewTrayHost()
	item, err := New(Options{Host: host})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	item.SetTitle("04:59")
	if got := host.Trays()[0].Title(); got != "04:59" {
		t.Fatalf("title = %q", got)
	}
	var none *Item
	none.SetTitle("ignored")
	none.Close()
}
