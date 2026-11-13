package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/hyprland"
	"github.com/pablopunk/nvm/native/internal/platform/instance"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/shortcuts/compositor"
)

// compositorService returns the Hyprland service, created on first use. Its state lives in the machine-local folder.
func (p *App) compositorService() *compositor.Service {
	if p.compositor == nil {
		home, _ := os.UserHomeDir()
		env := hyprland.Env{
			Getenv: os.Getenv, Home: home, CompositorArgs: hyprland.CompositorArgs,
			Run: p.runCompositorTool, UndoFile: filepath.Join(dataDir(), "hyprland-undo.json"),
		}
		p.compositor = compositor.New(env, filepath.Join(dataDir(), "hyprland.json"), time.Now)
	}
	return p.compositor
}

// runCompositorTool runs hyprctl or Hyprland through the commands port, with the user's login PATH.
func (p *App) runCompositorTool(ctx context.Context, name string, args ...string) ([]byte, int, error) {
	if p.ports.Commands == nil {
		return nil, 0, platform.ErrUnsupported
	}
	result, err := p.ports.Commands.Run(ctx, platform.CommandSpec{Binary: name, Args: args})
	return result.Stdout, result.ExitCode, err
}

// shortcutHotkeys is the hotkey port the shortcut manager binds through. In a Hyprland session the compositor's module
// may own the shortcuts; then the portal does not bind them as well.
func (p *App) shortcutHotkeys() platform.Hotkeys {
	if p.ports.Hotkeys == nil || os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return p.ports.Hotkeys
	}
	return p.compositorService().Hotkeys(p.ports.Hotkeys, p.requestCompositorSync)
}

// requestCompositorSync updates the module after the bound shortcuts change, once for a burst of changes.
func (p *App) requestCompositorSync() {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" || !p.compositorReady.Load() {
		return
	}
	p.compositorSync.request(p.onUI, p.syncCompositorModule)
}

// syncCompositorModule keeps the module in line with the shortcuts without being asked. It waits for the extensions so
// no extension shortcut is judged before it exists, and it runs on the UI thread. Success is silent; a problem is
// reported once (see reportCompositorSync).
func (p *App) syncCompositorModule() {
	svc := p.compositorService()
	if svc.State().Disabled || p.extensionManager() == nil {
		return
	}
	desired := p.compositorDesired()
	safego.Go("hyprland.sync", func(ctx context.Context) {
		outcome, err := svc.Sync(ctx, desired)
		p.onUI(func() { p.finishCompositorSync(outcome, err) })
	})
}

// finishCompositorSync rebinds the portal's shortcuts when the compositor took over, and reports a problem once.
func (p *App) finishCompositorSync(outcome compositor.SyncOutcome, err error) {
	if outcome.OwnershipChanged {
		p.resumeShortcutBindings()
	}
	if err != nil {
		logx.Default().Warn("hyprland.sync.failed", logx.F{"error": err.Error()})
	}
	if message := compositorProblem(outcome, err); p.compositorReported.shouldReport(message) {
		p.notifySystem("Hyprland shortcuts", message)
	}
}

// compositorProblem names what a sync could not do, in a short sentence, or "" when all went well.
func compositorProblem(outcome compositor.SyncOutcome, err error) string {
	switch {
	case err != nil:
		return "Not updated, your config was restored: " + err.Error()
	case len(outcome.Skipped) > 0:
		s := outcome.Skipped[0]
		return fmt.Sprintf("%s was not added: %s", s.What, s.Reason)
	case outcome.Result.Loaded == hyprland.LoadedMissing:
		return "Hyprland does not list the shortcuts after the reload; check the include in your config"
	}
	return ""
}

// resumeShortcutBindings binds the shortcuts again, so ownership moves between the portal and the compositor.
func (p *App) resumeShortcutBindings() {
	if p.shortcutMgr != nil && !p.shortcutMgr.Suspended() {
		p.shortcutMgr.Suspend()
		p.summonChanged(p.shortcutMgr.Resume())
	}
}

// compositorDesired is the module for the current shortcuts. It runs on the UI thread.
func (p *App) compositorDesired() hyprland.Desired {
	m := p.shortcutsManager()
	return compositor.DesiredFor(m.PaletteAccelerator(), p.runnableActions(m.Accelerators()), compositorIdentity())
}

// compositorIdentity is the executable bindings run and the app IDs of the app's windows: the executable's name, which
// MyGo uses as the Wayland app ID, and the installed command's name, used when a desktop entry starts it.
func compositorIdentity() compositor.Identity {
	exe, err := instance.Executable()
	if err != nil {
		return compositor.Identity{}
	}
	classes := []string{filepath.Base(exe)}
	if !slices.Contains(classes, appFolderSlug) {
		classes = append(classes, appFolderSlug)
	}
	return compositor.Identity{Executable: exe, Classes: classes}
}
