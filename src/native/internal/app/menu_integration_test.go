package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	menusbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/menus"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/menushost"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

type menuInvocation struct {
	source    platform.MenuSource
	reference string
}

type integrationMenuSource struct {
	captured      chan int
	reading       chan struct{}
	releaseRead   chan struct{}
	invoked       chan menuInvocation
	releaseInvoke chan struct{}
}

func (s *integrationMenuSource) Capture(_ context.Context, pid int) (platform.MenuSource, error) {
	s.captured <- pid
	return platform.MenuSource{PID: pid, Identity: "original-process"}, nil
}

func (s *integrationMenuSource) Snapshot(ctx context.Context, _ platform.MenuSource) ([]platform.MenuEntry, error) {
	s.reading <- struct{}{}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.releaseRead:
	}
	return []platform.MenuEntry{
		{Reference: "open-reference", Title: "Open Document", Path: []string{"File"}, Shortcut: "⌘O", Enabled: true, Checked: true},
		{Reference: "disabled-reference", Title: "Disabled Item", Path: []string{"File"}},
	}, nil
}

func (s *integrationMenuSource) Invoke(ctx context.Context, source platform.MenuSource, reference string) error {
	s.invoked <- menuInvocation{source, reference}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.releaseInvoke:
		return nil
	}
}

func TestMenuBuiltinThroughCapturedAppAndManager(t *testing.T) {
	p := goldenPalette(t)
	w := &queuedWindow{}
	p.Attach(w)
	p.uiQueue = w
	p.SetStack(&palette.View{Title: "Root"})
	source := &integrationMenuSource{make(chan int, 1), make(chan struct{}, 1), make(chan struct{}), make(chan menuInvocation, 1), make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notified := make(chan struct{}, 8)
	p.menus = menushost.New(ctx, source, time.Minute, func() { p.menuChanged(); notified <- struct{}{} })
	defer p.menus.Close()
	windows := p.ports.Windows.(*fake.WindowManager)
	original := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "original"})
	windows.Focus(original)
	p.captureMenusOnShow()
	<-notified
	if pid := <-source.captured; pid != 42 {
		t.Fatalf("captured PID %d", pid)
	}
	<-source.reading
	other := windows.AddWindow(fake.SimWindow{PID: 77, BundleID: "nevermind"})
	windows.Focus(other)
	w.Show()
	w.drain()
	m := newMenuTestManager(t, p)
	result, failure, err := m.Run(ctx, menusbuiltin.ID, api.Invocation{Ref: api.ActionRef{ID: menusbuiltin.CommandID}})
	if err != nil || failure != nil || result.View == nil || !result.View.IsLoading || len(result.View.Items) != 0 {
		t.Fatalf("held read did not return loading view: %+v %+v %v", result, failure, err)
	}
	session := extview.NewSession(p.extensionViewDeps(m), menusbuiltin.ID, *result.View)
	page := session.Open(p.Palette)
	p.Push(page)
	p.SetQuery("Open")
	p.settleExtensionCalls()
	w.drain()
	close(source.releaseRead)
	<-notified
	w.drain()
	p.settleExtensionCalls()
	w.drain()
	rows := page.Search("")
	if len(rows) != 2 || rows[0].Title != "Open Document" || rows[0].Subtitle != "File" || !rows[1].Disabled || len(rows[1].Actions) != 0 {
		t.Fatalf("host menu rows=%+v", rows)
	}
	if p.Query() != "Open" || len(p.Results()) != 1 || p.Results()[0].Title != "Open Document" || page.ExtensionID != menusbuiltin.ID {
		t.Fatalf("notification lost query/open page: query=%q rows=%v", p.Query(), rowTitles(p))
	}
	handle := result.View.Items
	if len(handle) != 0 {
		t.Fatal("initial view mutated")
	}
	snapshot, err := p.menuPort().Snapshot(ctx)
	if err != nil || len(snapshot.Items) != 2 || snapshot.Items[0].Handle == "open-reference" || !snapshot.Items[0].Checked || snapshot.Items[0].Shortcut != "⌘O" {
		t.Fatalf("menu metadata/opaque handle missing: %+v %v", snapshot, err)
	}
	p.RunAction(p.Results()[0].Actions[0])
	invocation := <-source.invoked
	if invocation.source.PID != 42 || invocation.source.Identity != "original-process" || invocation.reference != "open-reference" || !w.IsVisible() {
		t.Fatalf("invoked wrong source or hid early: %+v visible=%t", invocation, w.IsVisible())
	}
	close(source.releaseInvoke)
	p.settleExtensionCalls()
	w.drain()
	if w.IsVisible() {
		t.Fatal("successful invocation did not dismiss")
	}
	if err := p.menuPort().Invoke(ctx, snapshot.Items[0].Handle); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("dismissed source accepted handle: %v", err)
	}
}

func newMenuTestManager(t *testing.T, p *App) *manager.Manager {
	t.Helper()
	state, err := store.Open(p.ports.FS, p.ports.Clock, "/menu-test-state")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = state.Close() })
	m, err := manager.New(manager.Ports{Files: p.ports.FS, Clock: p.ports.Clock, Menus: p.menuPort()}, "/menu-extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{menusbuiltin.New(p.menuPort() != nil)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

type deniedMenuSource struct{ platform.CurrentAppMenus }

func (deniedMenuSource) Capture(context.Context, int) (platform.MenuSource, error) {
	return platform.MenuSource{}, &platform.WindowError{Code: platform.WindowErrorDenied}
}

func TestMenuPermissionGuidanceThroughManager(t *testing.T) {
	set := fake.NewSet()
	p := &App{ports: set.Ports()}
	changes := make(chan struct{}, 4)
	p.menus = menushost.New(context.Background(), deniedMenuSource{}, time.Minute, func() { changes <- struct{}{} })
	t.Cleanup(p.menus.Close)
	p.menus.Capture(42)
	<-changes
	<-changes
	m := newMenuTestManager(t, p)
	result, failure, err := m.Run(context.Background(), menusbuiltin.ID, api.Invocation{Ref: api.ActionRef{ID: menusbuiltin.CommandID}})
	if err != nil || failure != nil || result.View == nil || result.View.EmptyView == nil || result.View.EmptyView.Title != "Accessibility access needed" {
		t.Fatalf("native denial lost permission guidance: result=%+v failure=%+v err=%v", result, failure, err)
	}
}

func TestMenuQuitCancelsCapturedSource(t *testing.T) {
	p := goldenPalette(t)
	source := &menuCapture{make(chan int, 1), make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.menuCancel = cancel
	p.menus = menushost.New(ctx, source, time.Minute, nil)
	p.menus.Capture(42)
	<-source.calls
	p.beginQuit(nil)
	<-source.cancelled
	if !p.quitting || ctx.Err() == nil {
		t.Fatal("quit did not cancel app-owned menu lifetime")
	}
	if _, err := p.menuPort().Snapshot(context.Background()); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("quit retained menu source: %v", err)
	}
}

func TestMenuBuiltinRegistrationFollowsCapability(t *testing.T) {
	for _, available := range []bool{false, true} {
		count := 0
		for _, extension := range builtInExtensions(builtInInputs{MenusAvailable: available}) {
			if extension.Manifest().ID != menusbuiltin.ID {
				continue
			}
			count++
			commands, err := extension.(interface {
				Commands(api.Context) ([]api.Command, error)
			}).Commands(nil)
			want := 0
			if available {
				want = 1
			}
			if err != nil || len(commands) != want || (available && commands[0].ID != menusbuiltin.CommandID) {
				t.Fatalf("available=%t commands=%+v err=%v", available, commands, err)
			}
		}
		if count != 1 {
			t.Fatalf("registered %d menu builtins", count)
		}
	}
}
