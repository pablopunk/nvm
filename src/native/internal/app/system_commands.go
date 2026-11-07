package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// runSystemOp runs the OS command an extension names, off the UI thread. done runs on the UI thread with the sentence to
// show when the command did not run, and "" when the OS accepted it.
func (p *App) runSystemOp(op string, done func(failure string)) {
	if op == appcommands.QuitOp {
		p.requestQuit()
		done("")
		return
	}
	id, ok := systemfeat.CommandForOp(op)
	if !ok {
		done(unknownSystemOpMessage)
		return
	}
	safego.Go("system."+op, func(context.Context) {
		failure := ""
		if err := p.systemSvc.Run(id); err != nil {
			failure = describeSystemFailure(p.systemSvc.CommandTitle(id), err)
		}
		p.onUI(func() { done(failure) })
	})
}

// unknownSystemOpMessage is what an OS command the host does not run says.
const unknownSystemOpMessage = "This system command is not available"

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
