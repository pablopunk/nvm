package extitems

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// serialUI is the test's UI thread: applies run one at a time, as they do on the app's UI thread, so a test's state is
// never written by two applies at once.
func serialUI() func(fn func()) {
	return func(fn func()) {
		uiThread.Lock()
		defer uiThread.Unlock()
		fn()
	}
}

// uiThread is the one lock every test dispatcher takes, so the scheduler and the service share one UI thread.
var uiThread sync.Mutex

// fakeSource is an extension host that answers from fields the test sets. Runs are reported on runs.
type fakeSource struct {
	mu        sync.Mutex
	ids       []string
	revision  string
	bars      []api.MenuBar
	failure   *api.Failure
	renderErr error
	triggers  []api.Trigger
	runs      chan api.Invocation
}

func (s *fakeSource) set(update func(*fakeSource)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(s)
}

func (s *fakeSource) MenuBarExtensions() []Extension {
	s.mu.Lock()
	defer s.mu.Unlock()
	var listed []Extension
	for _, id := range s.ids {
		listed = append(listed, Extension{ID: id, Revision: s.revision})
	}
	return listed
}

func (s *fakeSource) MenuBars(context.Context, string) ([]api.MenuBar, *api.Failure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bars, s.failure, s.renderErr
}

func (s *fakeSource) MenuBarTriggers(context.Context, string) ([]api.Trigger, *api.Failure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.triggers, nil, nil
}

func (s *fakeSource) Run(_ context.Context, _ string, in api.Invocation) (api.Result, *api.Failure, error) {
	s.runs <- in
	return api.Result{}, nil, nil
}

type harness struct {
	t      *testing.T
	clock  *fake.Clock
	tray   *fake.TrayHost
	sched  *menubar.Scheduler
	source *fakeSource
	svc    *Service
	opened chan string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	clock := fake.NewClock()
	tray := fake.NewTrayHost()
	sched := menubar.NewScheduler(clock, serialUI())
	source := &fakeSource{
		ids:      []string{"acme.menubar"},
		bars:     []api.MenuBar{clockBar("tick 01")},
		triggers: []api.Trigger{{Type: api.TriggerInterval, Every: "1s"}},
		runs:     make(chan api.Invocation, 8),
	}
	h := &harness{t: t, clock: clock, tray: tray, sched: sched, source: source, opened: make(chan string, 4)}
	h.svc = New(Options{
		Host:      tray,
		Scheduler: sched,
		OnUI:      serialUI(),
		Source:    source,
		Hooks: Hooks{
			OpenURL:        func(url string) { h.opened <- url },
			ShowExtensions: func(id string) { h.opened <- "extensions:" + id },
		},
		Logs: quietLogs(t, clock),
	})
	h.svc.Start()
	h.settle()
	return h
}

// settle runs the due jobs until nothing new starts, waiting for each run to apply.
func (h *harness) settle() {
	h.t.Helper()
	for range 4 {
		h.sched.Tick()
		h.sched.Wait()
	}
}

// advance moves time forward and lets every job that is now due run.
func (h *harness) advance(d time.Duration) {
	h.clock.Advance(d)
	h.settle()
}

func (h *harness) item(bar string) *fake.Tray {
	h.t.Helper()
	for _, tray := range h.tray.Trays() {
		if tray.Options().ID == "ext:acme.menubar:"+bar && !tray.Closed() {
			return tray
		}
	}
	h.t.Fatalf("no open item for bar %q", bar)
	return nil
}

func clockBar(title string) api.MenuBar {
	return api.MenuBar{
		ID: "main", Icon: "timer", Title: title, Tooltip: "Clock",
		Sections: []api.MenuSection{
			{Title: "Counter", Rows: []api.MenuRow{
				{ID: "count", Title: "Count 1", Disabled: true},
				{ID: "bump", Title: "Bump", Action: runAction("bump")},
			}},
			{Rows: []api.MenuRow{{ID: "more", Title: "More", Items: []api.MenuRow{
				{ID: "docs", Title: "Documentation", Action: openURLAction("https://example.com")},
			}}}},
		},
	}
}

func runAction(id string) *api.Action {
	action := api.Run(id, "Run", nil)
	return &action
}

func openURLAction(url string) *api.Action {
	action := api.OpenURL("Open", url)
	return &action
}

func quietLogs(t *testing.T, clock *fake.Clock) *logx.Logger {
	t.Helper()
	set := fake.NewSet()
	logs, err := logx.New(set.FS, clock, "/logs", logx.Options{})
	if err != nil {
		t.Fatalf("logx.New: %v", err)
	}
	return logs
}

func TestEnabledExtensionShowsItsItemWithTitleAndRows(t *testing.T) {
	h := newHarness(t)
	item := h.item("main")
	if item.Title() != "tick 01" {
		t.Fatalf("title = %q", item.Title())
	}
	rows := item.Items()
	if len(rows) != 5 || !rows[0].Disabled || rows[0].Label != "Counter" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[2].Action == nil {
		t.Fatal("the bump row has no action")
	}
	if len(rows[4].Submenu) != 1 {
		t.Fatalf("the submenu was not built: %+v", rows[4])
	}
}

func TestTitleFollowsTheCadenceOfItsInterval(t *testing.T) {
	h := newHarness(t)
	h.source.set(func(s *fakeSource) { s.bars = []api.MenuBar{clockBar("tick 02")} })
	h.advance(time.Second)
	if got := h.item("main").Title(); got != "tick 02" {
		t.Fatalf("title after one second = %q", got)
	}
}

func TestRefreshRerendersNowWithoutWaitingForTheInterval(t *testing.T) {
	h := newHarness(t)
	h.source.set(func(s *fakeSource) {
		s.triggers = nil
		s.bars = []api.MenuBar{clockBar("on demand")}
	})
	h.svc.Refresh("acme.menubar")
	h.settle()
	if got := h.item("main").Title(); got != "on demand" {
		t.Fatalf("title after refresh = %q", got)
	}
}

func TestSyncShowsAnExtensionThatStartedWithoutWaitingForThePeriodicSync(t *testing.T) {
	h := newHarness(t)
	h.source.set(func(s *fakeSource) { s.ids = nil })
	h.svc.Sync()
	h.settle()
	h.source.set(func(s *fakeSource) {
		s.ids = []string{"acme.menubar"}
		s.triggers = nil
		s.bars = []api.MenuBar{clockBar("started")}
	})
	h.svc.Sync()
	h.settle()
	if got := h.item("main").Title(); got != "started" {
		t.Fatalf("title after a lifecycle sync = %q", got)
	}
}

func TestRenderFailureKeepsTheLastGoodRowsUnderANotRespondingHead(t *testing.T) {
	h := newHarness(t)
	h.source.set(func(s *fakeSource) { s.renderErr = errors.New("boom") })
	h.advance(time.Second)
	rows := h.item("main").Items()
	if !strings.HasPrefix(rows[0].Label, "Not responding: boom") || !rows[0].Disabled {
		t.Fatalf("head = %+v", rows[0])
	}
	if rows[1].Label != "Show in Extensions" || rows[1].Action == nil {
		t.Fatalf("the show row is missing: %+v", rows[1])
	}
	if got := h.item("main").Title(); got != "tick 01" {
		t.Fatalf("a failing render changed the title to %q", got)
	}
	h.source.set(func(s *fakeSource) { s.renderErr = nil })
	h.advance(time.Second)
	if strings.HasPrefix(h.item("main").Items()[0].Label, "Not responding") {
		t.Fatal("the head stayed after the render recovered")
	}
}

func TestFailureWithoutAnyGoodRenderStillShowsAnItem(t *testing.T) {
	clock := fake.NewClock()
	tray := fake.NewTrayHost()
	sched := menubar.NewScheduler(clock, serialUI())
	source := &fakeSource{ids: []string{"acme.menubar"}, failure: &api.Failure{Code: api.CodeInternal, Message: "never rendered"}, runs: make(chan api.Invocation, 1)}
	svc := New(Options{Host: tray, Scheduler: sched, OnUI: serialUI(), Source: source, Logs: quietLogs(t, clock)})
	svc.Start()
	for range 4 {
		sched.Tick()
		sched.Wait()
	}
	trays := tray.Trays()
	if len(trays) != 1 || !strings.HasPrefix(trays[0].Items()[0].Label, "Not responding: never rendered") {
		t.Fatalf("trays = %+v", trays)
	}
}

func TestNewVersionReadsItsTriggersAgain(t *testing.T) {
	h := newHarness(t)
	h.source.set(func(s *fakeSource) {
		s.revision = "v2"
		s.triggers = nil
		s.bars = []api.MenuBar{clockBar("v2")}
	})
	h.advance(time.Second)
	if got := h.item("main").Title(); got != "v2" {
		t.Fatalf("title after the new version = %q", got)
	}
	h.source.set(func(s *fakeSource) { s.bars = []api.MenuBar{clockBar("no cadence")} })
	h.advance(time.Hour)
	if got := h.item("main").Title(); got != "v2" {
		t.Fatalf("the old interval still re-renders the new version: %q", got)
	}
}

func TestDisablingTheExtensionClosesItsItems(t *testing.T) {
	h := newHarness(t)
	item := h.item("main")
	h.source.set(func(s *fakeSource) { s.ids = nil })
	h.advance(time.Second)
	if !item.Closed() {
		t.Fatal("the item of a disabled extension is still open")
	}
}

func TestClickingARunRowRunsTheExtensionActionOffTheUIThread(t *testing.T) {
	h := newHarness(t)
	if err := h.item("main").Invoke("0.1"); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	select {
	case in := <-h.source.runs:
		if in.Ref.ID != "bump" {
			t.Fatalf("ran %q, want bump", in.Ref.ID)
		}
	case <-time.After(waitFor):
		t.Fatal("the run never reached the extension")
	}
}

func TestOpenURLRowUsesTheHostHookWithoutTheExtension(t *testing.T) {
	h := newHarness(t)
	h.item("main").Items()[4].Submenu[0].Action()
	select {
	case url := <-h.opened:
		if url != "https://example.com" {
			t.Fatalf("opened %q", url)
		}
	case <-time.After(waitFor):
		t.Fatal("the URL hook did not run")
	}
}

func TestUnavailableTriggerIsDisabledAndTheItemStillShows(t *testing.T) {
	clock := fake.NewClock()
	tray := fake.NewTrayHost()
	sched := menubar.NewScheduler(clock, serialUI())
	source := &fakeSource{
		ids:      []string{"acme.menubar"},
		bars:     []api.MenuBar{clockBar("wake")},
		triggers: []api.Trigger{{Type: api.TriggerWake}},
		runs:     make(chan api.Invocation, 1),
	}
	svc := New(Options{Host: tray, Scheduler: sched, OnUI: serialUI(), Source: source, Logs: quietLogs(t, clock)})
	svc.Start()
	for range 4 {
		sched.Tick()
		sched.Wait()
	}
	clock.Advance(time.Hour)
	sched.Tick()
	sched.Wait()
	if trays := tray.Trays(); len(trays) != 1 || trays[0].Title() != "wake" {
		t.Fatalf("trays = %+v", trays)
	}
}

func TestTrayFailureLeavesTheServiceRunning(t *testing.T) {
	clock := fake.NewClock()
	tray := fake.NewTrayHost()
	tray.FailNext(fake.OpTrayNewTray, errors.New("no tray host"))
	sched := menubar.NewScheduler(clock, serialUI())
	source := &fakeSource{ids: []string{"acme.menubar"}, bars: []api.MenuBar{clockBar("x")}, runs: make(chan api.Invocation, 1)}
	svc := New(Options{Host: tray, Scheduler: sched, OnUI: serialUI(), Source: source, Logs: quietLogs(t, clock)})
	svc.Start()
	for range 4 {
		sched.Tick()
		sched.Wait()
	}
	if len(tray.Trays()) != 0 {
		t.Fatal("an item was created after the tray host refused")
	}
}

// waitFor bounds the waits on a channel that a successful run fills at once.
const waitFor = 20 * time.Second
