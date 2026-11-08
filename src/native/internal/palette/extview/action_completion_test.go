package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

type actionWindow struct {
	editWindow
	visible bool
}

func (w *actionWindow) Show()           { w.visible = true }
func (w *actionWindow) Hide()           { w.visible = false }
func (w *actionWindow) IsVisible() bool { return w.visible }

func TestDismissedCommandCompletesWindowEffectWithoutClosingANewerPalette(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		auto, nested, reopen bool
	}{
		{name: "root automatic hide", auto: true},
		{name: "view automatic hide", auto: true, nested: true},
		{name: "ignore after resummon", auto: true, reopen: true},
		{name: "ignore after manual hide"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var done func(api.Result, *api.Failure)
			windows := 0
			deps := Deps{
				Invoke: func(_ string, _ api.Invocation, callback func(api.Result, *api.Failure)) { done = callback },
				Window: func(string, api.Action) { windows++ },
			}
			action := api.Run("toggle", "Toggle floating window", nil)
			action.DismissAfterRun = tc.auto
			root := &palette.View{Title: "root", Search: func(string) []search.Item {
				return []search.Item{{Title: action.Title, Actions: []search.Action{ActionFor(deps, "ticker", action)}}}
			}}
			p := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
			window := &actionWindow{}
			p.Attach(window)
			p.Show()
			if tc.nested {
				session := NewSession(deps, "ticker", windowRowView(action))
				p.Push(session.Open(p))
			}
			tester := ui.NewTester(p.View, 720, 470)
			tester.Frame()
			tester.Key(0, ui.KeyEnter)
			tester.Frame()
			if done == nil || window.visible == tc.auto {
				t.Fatal("command did not dispatch and immediately hide")
			}
			if !tc.auto {
				p.Hide()
			}
			if tc.reopen {
				p.Show()
			}
			next := api.Action{Kind: api.ActionWindow, Op: "toggle", Window: &api.WindowRef{ID: "floating"}}
			done(api.Result{Next: &next}, nil)
			if tc.reopen || !tc.auto {
				if windows != 0 || window.visible != tc.reopen {
					t.Fatal("obsolete completion disturbed the newly summoned palette")
				}
			} else if windows != 1 {
				t.Fatal("automatic dismissal discarded the command's window effect")
			}
		})
	}
}

func TestInitialActionLateMutationSurvivesDismissal(t *testing.T) {
	identity := new(int)
	cache := NewMemoryCache()
	lease := cache.acquire("test", identity)
	before := lease.version()
	var done func(api.Result, *api.Failure)
	deps := Deps{
		Cache:                   cache,
		CommandSnapshotIdentity: func(string) any { return identity },
		InvokeViewAction:        func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
	}
	session := NewSession(deps, "test", api.View{})
	host := &fakeHost{}
	session.action(api.Run("write", "Write", nil), "row").Run(host)
	if done == nil {
		t.Fatal("action was not dispatched")
	}
	session.close()
	view := api.ListView("late")
	done(api.Result{DataChanged: true, View: &view, OriginIdentity: identity, OriginBound: true}, nil)
	if len(host.pushed) != 0 || lease.version() == before {
		t.Fatal("dismissal lost mutation or presented stale view")
	}
	lease.release()
}
