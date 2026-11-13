package app

import (
	"context"
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
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return
	}
	p.compositorSync.request(p.onUI, p.syncCompositorModule)
}

// syncCompositorModule rewrites the module when setup is done and the shortcuts no longer match it. It never adds the
// include, and it waits for the extensions so no extension shortcut is judged before they exist. A failure is shown as
// a notification and on the Hyprland page.
func (p *App) syncCompositorModule() {
	svc := p.compositorService()
	if !svc.State().OwnsShortcuts || p.extensionManager() == nil {
		return
	}
	desired := p.compositorDesired()
	safego.Go("hyprland.sync", func(ctx context.Context) {
		plan, err := svc.Preview(ctx, desired)
		if err == nil && plan.ModuleUnchanged() {
			return
		}
		if err == nil {
			_, err = svc.Apply(ctx, plan, false)
		}
		if err != nil {
			logx.Default().Warn("hyprland.sync.failed", logx.F{"error": err.Error()})
			p.onUI(func() { p.notifySystem("Hyprland shortcuts not updated", err.Error()) })
		}
	})
}

// compositorDesired is the module for the current shortcuts and appearance choice. It runs on the UI thread.
func (p *App) compositorDesired() hyprland.Desired {
	m := p.shortcutsManager()
	return compositor.DesiredFor(m.PaletteAccelerator(), p.runnableActions(m.Accelerators()), compositorIdentity(), p.compositorService().State().Undecorated)
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
