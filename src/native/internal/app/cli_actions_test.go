package app

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
	"github.com/pablopunk/nvm/native/internal/store"
)

// The command line runs an action as its shortcut does and never runs a substitute for an unknown one.
func TestActionCommandOutcomes(t *testing.T) {
	p := goldenPalette(t)
	p.Attach(&testWindow{})
	if r := p.runActionCommandOnUI(shortcuts.PaletteID); r.Outcome != actioncli.Opened || !p.Visible() {
		t.Fatalf("palette: %+v visible=%v", r, p.Visible())
	}
	if r := p.runActionCommandOnUI(shortcuts.CommandKeyboardShortcuts); r.Outcome != actioncli.Opened {
		t.Fatalf("keyboard shortcuts: %+v", r)
	}
	for _, id := range []string{"command:no-such-command", "extension:gone.example:run", "unknown"} {
		r := p.runActionCommandOnUI(id)
		if r.Outcome != actioncli.Rejected || r.Message == "" {
			t.Errorf("%s: %+v", id, r)
		}
	}
}

func TestPaletteRowCopiesTheToggleCommand(t *testing.T) {
	if got := actionCommandLine(shortcuts.PaletteID); !strings.HasSuffix(got, " run palette-toggle") {
		t.Fatalf("palette command line %q", got)
	}
	if got := actionCommandLine(shortcuts.CommandQuickNotes); !strings.HasSuffix(got, " run command:quick-notes") {
		t.Fatalf("quick notes command line %q", got)
	}
}

// flakyShortcutProbe fails its first listing, as an extension that is still starting does. The manager then holds off
// its own retries for a while, so a request cannot rely on the cache catching up.
type flakyShortcutProbe struct {
	shortcutCommandProbe
	listed *atomic.Int32
}

func (p flakyShortcutProbe) Commands(ctx api.Context) ([]api.Command, error) {
	if p.listed.Add(1) == 1 {
		return nil, errors.New("still starting")
	}
	return p.shortcutCommandProbe.Commands(ctx)
}

// On a cold start the cache has not learned the extension's commands, and its own discovery failed once. The request
// must still find the command, through the real manager, not a fake list.
func TestRunFindsExtensionCommandBeforeTheCacheLearnedIt(t *testing.T) {
	services := fake.NewSet()
	state, err := store.Open(services.FS, services.Clock, "/state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	calls := make(chan api.Invocation, 1)
	probe := flakyShortcutProbe{shortcutCommandProbe{calls: calls, mode: api.ModeBackground, identity: "nevermind.slow-probe", actionID: "extension:nevermind.slow-probe:declared"}, new(atomic.Int32)}
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{probe}})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	app := &App{Palette: palette.New(palette.Options{}), ports: services.Ports()}
	app.exts.mgr.Store(m)
	app.exts.settled.settle()
	m.CachedCommands() // the cold start's own discovery begins, as the first search does
	for probe.listed.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if r := app.runActionCommand(ctx, "extension:nevermind.slow-probe:declared"); r.Outcome != actioncli.Started {
		t.Fatalf("cold run: %+v", r)
	}
	select {
	case in := <-calls:
		if in.Ref.ID != "declared" {
			t.Fatalf("invocation %#v", in)
		}
	case <-ctx.Done():
		t.Fatal("the command never ran")
	}
}
