package manager

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// menuBarHarness is a manager with the menu bar fixture installed, enabled and running. Its refresh calls are recorded.
type menuBarHarness struct {
	*harness
	mu        sync.Mutex
	refreshed []string
}

func newMenuBarHarness(t *testing.T) *menuBarHarness {
	t.Helper()
	mb := &menuBarHarness{}
	h := newHarness(t, func(o *Options) {
		o.MenuBarRefresh = func(id string) {
			mb.mu.Lock()
			defer mb.mu.Unlock()
			mb.refreshed = append(mb.refreshed, id)
		}
	})
	mb.harness = h
	if err := h.m.Install("acme.menubar", fixtureContents(t, "acme.menubar"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.menubar")
	return mb
}

func (mb *menuBarHarness) refreshes() []string {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	return append([]string(nil), mb.refreshed...)
}

func (mb *menuBarHarness) menuBars(t *testing.T) ([]api.MenuBar, *api.Failure) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	bars, failure, err := mb.m.MenuBars(ctx, "acme.menubar")
	if err != nil {
		t.Fatalf("MenuBars: %v", err)
	}
	return bars, failure
}

func (mb *menuBarHarness) run(t *testing.T, action string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	_, failure, err := mb.m.Run(ctx, "acme.menubar", api.Invocation{Ref: api.ActionRef{ID: action}})
	if err != nil || failure != nil {
		t.Fatalf("Run %s: err %v failure %+v", action, err, failure)
	}
}

func TestEnabledMenuBarExtensionIsListedAndRendersItsItems(t *testing.T) {
	mb := newMenuBarHarness(t)
	if listed := mb.m.MenuBarExtensions(); len(listed) != 1 || listed[0].ID != "acme.menubar" || listed[0].Revision == "" {
		t.Fatalf("MenuBarExtensions = %+v", listed)
	}
	bars, failure := mb.menuBars(t)
	if failure != nil || len(bars) != 2 || bars[0].ID != "main" || bars[1].ID != "status" {
		t.Fatalf("MenuBars = %+v, failure %+v", bars, failure)
	}
	if len(bars[0].Sections) != 3 || bars[0].Sections[2].Rows[0].Items == nil {
		t.Fatalf("the main item lost its sections or submenu: %+v", bars[0].Sections)
	}
}

func TestMenuBarTriggersAreReadFromTheSource(t *testing.T) {
	mb := newMenuBarHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	triggers, failure, err := mb.m.MenuBarTriggers(ctx, "acme.menubar")
	if err != nil || failure != nil {
		t.Fatalf("MenuBarTriggers: err %v failure %+v", err, failure)
	}
	if len(triggers) != 2 || triggers[0].Type != api.TriggerInterval || triggers[0].Every != "1s" {
		t.Fatalf("triggers = %+v", triggers)
	}
}

func TestFailingRenderReturnsAFailureAndFixRecovers(t *testing.T) {
	mb := newMenuBarHarness(t)
	mb.run(t, "break")
	_, failure := mb.menuBars(t)
	if failure == nil || !strings.Contains(failure.Message, "fixture was told to fail") {
		t.Fatalf("failure = %+v; want the render error", failure)
	}
	mb.run(t, "fix")
	if _, failure := mb.menuBars(t); failure != nil {
		t.Fatalf("after fix the render still fails: %+v", failure)
	}
}

func TestRefreshMenuBarCallReachesTheHostHook(t *testing.T) {
	mb := newMenuBarHarness(t)
	mb.run(t, "bump")
	if got := mb.refreshes(); len(got) != 1 || got[0] != "acme.menubar" {
		t.Fatalf("refresh calls = %v; want one for acme.menubar", got)
	}
}

func TestDisabledMenuBarExtensionIsNotListed(t *testing.T) {
	mb := newMenuBarHarness(t)
	if err := mb.m.Disable("acme.menubar"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if listed := mb.m.MenuBarExtensions(); len(listed) != 0 {
		t.Fatalf("disabled extension still listed: %+v", listed)
	}
	if mb.m.List()[0].Runtime.State != supervisor.StateIdle {
		t.Fatalf("disabled runtime state = %v", mb.m.List()[0].Runtime.State)
	}
}
