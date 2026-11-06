package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/diagnostics"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
)

type shortcutCommandProbe struct {
	calls    chan api.Invocation
	mode     api.BackgroundMode
	actionID string
	identity string
}

func (p shortcutCommandProbe) Manifest() api.Manifest {
	id := p.identity
	if id == "" {
		id = "nevermind.shortcut-probe"
	}
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: id, Title: "Shortcut probe"}}
}

func (p shortcutCommandProbe) Commands(api.Context) ([]api.Command, error) {
	actionID := p.actionID
	if actionID == "" {
		actionID = "extension-action:nevermind.shortcut-probe:declared"
	}
	return []api.Command{{ID: "declared", Title: "Declared primary", ActionID: actionID, Mode: p.mode, DismissAfterRun: true}}, nil
}

func (p shortcutCommandProbe) RunAction(_ api.Context, in api.Invocation) (api.Result, error) {
	p.calls <- in
	return api.Result{}, nil
}

func TestShortcutActionUsesCachedCommandDeclarations(t *testing.T) {
	services := fake.NewSet()
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	calls := make(chan api.Invocation, 2)
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{diagnostics.New(), shortcutCommandProbe{calls: calls, mode: api.ModeBackground}}})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	results := m.Commands(context.Background())
	if len(results) != 2 {
		t.Fatalf("discovery = %#v", results)
	}
	for _, result := range results {
		if result.Failure != nil || len(result.Commands) != 1 {
			t.Fatalf("discovery = %#v", results)
		}
	}
	app := &App{Palette: palette.New(palette.Options{}), ports: services.Ports()}
	app.exts.mgr.Store(m)
	if len(app.globalShortcuts.commands) != 0 || len(m.CachedCommands()) != 2 {
		t.Fatal("fixture requires cached declarations before the UI shortcut table is published")
	}

	for _, id := range []string{"extension-action:nevermind.diagnostics:open", "extension:nevermind.shortcut-probe:missing", "extension:missing:declared"} {
		if _, ok := app.shortcutAction(id); ok {
			t.Errorf("undeclared shortcut %q resolved", id)
		}
	}
	select {
	case in := <-calls:
		t.Fatalf("rejected shortcut invoked extension: %#v", in)
	default:
	}

	for _, id := range []string{shortcuts.CommandReportProblem, "extension:nevermind.diagnostics:report-problem", "extension-action:nevermind.diagnostics:report-problem"} {
		action, ok := app.shortcutAction(id)
		if !ok || action.Title != "Report a Problem" || action.Run == nil {
			t.Errorf("report shortcut %q = %#v, %v", id, action, ok)
		}
	}
	for _, id := range []string{"extension:nevermind.shortcut-probe:declared", "extension-action:nevermind.shortcut-probe:declared"} {
		action, ok := app.shortcutAction(id)
		if !ok || action.Title != "Declared primary" || action.Run == nil || action.Dismiss != search.DismissAuto {
			t.Fatalf("shortcut %q = %#v, %v", id, action, ok)
		}
		action.Run(commandMenuHost{})
		in := <-calls
		app.settleExtensionCalls()
		if in.Ref.ID != "declared" || len(in.Ref.Args) != 0 || !in.Background {
			t.Errorf("shortcut %q invocation = %#v", id, in)
		}
	}
}

type shortcutRunObserver struct{ finished chan struct{} }

func (o shortcutRunObserver) BeginRun(ctx context.Context) (context.Context, func(manager.RunOutcome)) {
	return ctx, func(manager.RunOutcome) { o.finished <- struct{}{} }
}

func TestRunShortcutCommandUsesCachedMode(t *testing.T) {
	for _, mode := range []api.BackgroundMode{api.ModeNoView, api.ModeBackground} {
		t.Run(string(mode), func(t *testing.T) {
			services := fake.NewSet()
			state, err := store.Open(services.FS, services.Clock, "/state")
			if err != nil {
				t.Fatal(err)
			}
			defer state.Close()
			calls := make(chan api.Invocation, 2)
			finished := make(chan struct{}, 2)
			m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{shortcutCommandProbe{calls: calls, mode: mode, actionID: "custom:declared"}}, RunObserver: shortcutRunObserver{finished: finished}})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			results := m.Commands(context.Background())
			if len(results) != 1 || results[0].Failure != nil || len(m.CachedCommands()) != 1 {
				t.Fatalf("discovery: %#v", results)
			}
			app := &App{ports: services.Ports()}
			app.exts.mgr.Store(m)
			if len(app.globalShortcuts.commands) != 0 {
				t.Fatal("UI table must be cold")
			}
			app.runShortcutCommand("custom:declared")
			in := <-calls
			<-finished
			if in.Ref.ID != "declared" || in.Background != (mode == api.ModeBackground) {
				t.Fatalf("invocation: %#v", in)
			}
			select {
			case extra := <-calls:
				t.Fatalf("extra invocation: %#v", extra)
			default:
			}
		})
	}
}

func TestCachedShortcutCollisionMatchesPublishedTable(t *testing.T) {
	services := fake.NewSet()
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	probes := []api.Extension{
		shortcutCommandProbe{identity: "nevermind.collision-first", actionID: "shared:run", mode: api.ModeNoView},
		shortcutCommandProbe{identity: "nevermind.collision-second", actionID: "shared:run", mode: api.ModeBackground},
		shortcutCommandProbe{identity: "nevermind.collision-alias", actionID: "different:key", mode: api.ModeNoView},
		shortcutCommandProbe{identity: "nevermind.collision-exact", actionID: "extension:nevermind.collision-alias:declared", mode: api.ModeBackground},
	}
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: probes})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	results := m.Commands(context.Background())
	if len(results) != len(probes) {
		t.Fatalf("discovery: %#v", results)
	}
	for _, result := range results {
		if result.Failure != nil || len(result.Commands) != 1 {
			t.Fatalf("discovery: %#v", results)
		}
	}
	app := &App{ports: services.Ports()}
	app.exts.mgr.Store(m)
	table := globalShortcutTableOf(m.CachedCommands())
	for _, key := range []string{"shared:run", "extension:nevermind.collision-alias:declared"} {
		want, present := table.commands[key]
		got, ok := app.extensionShortcutCommand(key)
		if !present || !ok || got.extID != want.extID || got.command.Mode != want.command.Mode {
			t.Fatalf("%s: got %#v (%v), table %#v (%v)", key, got, ok, want, present)
		}
		if key == "extension:nevermind.collision-alias:declared" && got.extID != "nevermind.collision-exact" {
			t.Fatalf("exact key lost to alias: %#v", got)
		}
	}
}
