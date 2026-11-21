package app

import (
	"context"
	"os"

	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// hyprlandCommandID names the command that removes or restores Nevermind's Hyprland config.
const hyprlandCommandID = "command:hyprland"

// hyprlandCommands offers the way out of the automatic Hyprland config, only in a Hyprland session and only by
// search: Nevermind manages the config on its own, so there is no root row. The command removes the module and the one
// include line it added, and after that Nevermind leaves the config alone until the command restores it.
func (p *App) hyprlandCommands() []command {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil
	}
	svc := p.compositorService()
	if svc.State().Disabled {
		return []command{p.hyprlandCommand("Set Up Hyprland Config", "Write Nevermind's shortcuts and window rule into your Hyprland config again", p.restoreHyprlandConfig)}
	}
	return []command{p.hyprlandCommand("Remove Hyprland Config", "Delete Nevermind's module and include from your Hyprland config", p.removeHyprlandConfig)}
}

func (p *App) hyprlandCommand(title, subtitle string, run func(search.Host)) command {
	return command{
		ID: hyprlandCommandID, Title: title, Icon: "monitor", Hidden: true, Dismiss: search.DismissNever, Run: run,
		Keywords: "hyprland compositor shortcuts keybinds window rules borders config remove undo uninstall", Subtitle: subtitle,
	}
}

func (p *App) removeHyprlandConfig(h search.Host) {
	safego.Go("hyprland.remove", func(ctx context.Context) {
		_, err := p.compositorService().Remove(ctx)
		p.onUI(func() {
			p.resumeShortcutBindings()
			h.Toast(hyprlandStepMessage(err, "Removed Nevermind's Hyprland config"))
		})
	})
}

func (p *App) restoreHyprlandConfig(h search.Host) {
	if err := p.compositorService().Enable(); err != nil {
		h.Toast(err.Error())
		return
	}
	p.requestCompositorSync()
	h.Toast("Setting up the Hyprland config")
}

func hyprlandStepMessage(err error, done string) string {
	if err != nil {
		return err.Error()
	}
	return done
}
