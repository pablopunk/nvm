package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// systemCommands turns the system commands and the home folders into palette commands. They stay out of the root
// suggestions, as the other OS commands do; they are found by search.
func (p *App) systemCommands() []command {
	var out []command
	for _, c := range p.systemSvc.Commands() {
		out = append(out, p.systemCommand(c))
	}
	return out
}

// placeCommands lists the home folders as palette commands, after the other commands, as they always were.
func (p *App) placeCommands() []command {
	var out []command
	for _, place := range p.systemSvc.Places() {
		out = append(out, p.placeCommand(place))
	}
	return out
}

// systemCommand maps one system command onto the palette. A destructive command asks first, in a confirmation view;
// the others hide the palette and run.
func (p *App) systemCommand(c systemfeat.Command) command {
	return command{
		Title: c.Title, Subtitle: c.Subtitle, Keywords: c.Keywords, Icon: c.Icon, Tone: c.Tone, Hidden: true,
		Run: func(h search.Host) {
			if c.Destructive {
				h.Push(palette.Confirm(c.Title, c.Confirm, func() { p.runSystemCommand(c) }))
				return
			}
			h.Hide()
			p.runSystemCommand(c)
		},
	}
}

// placeCommand opens one home folder in the file manager.
func (p *App) placeCommand(place systemfeat.Place) command {
	return command{
		Title: place.Name, Subtitle: place.Path, Keywords: "open folder places go", Icon: "folder", Tone: "blue", Hidden: true,
		Run: func(h search.Host) {
			h.Hide()
			_ = p.systemSvc.OpenPlace(place)
		},
	}
}

// runSystemCommand performs a command off the UI thread. Failures reach the user as a toast; success needs none,
// since the OS acts on its own.
func (p *App) runSystemCommand(c systemfeat.Command) {
	safego.Go("system."+string(c.ID), func(context.Context) {
		err := p.systemSvc.Run(c.ID)
		if err == nil {
			return
		}
		p.toastFromWorker(describeSystemFailure(c.Title, err))
	})
}

// toastFromWorker shows a message from a background goroutine, on the UI thread. Without a panel (the goldens) the
// message has nowhere to go and is dropped.
func (p *App) toastFromWorker(msg string) {
	if p.panel == nil {
		return
	}
	p.panel.Update(func() { p.Toast(msg) })
}

// describeSystemFailure is the one sentence that says why a command did not run. An unsupported intent says so
// plainly; anything else keeps the OS's own reason.
func describeSystemFailure(title string, err error) string {
	if errors.Is(err, platform.ErrUnsupported) {
		return title + " is not available on this system."
	}
	return fmt.Sprintf("Could not run %s: %s", strings.ToLower(title), err)
}
