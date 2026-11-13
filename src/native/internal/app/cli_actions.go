package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform/instance"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/shortcuts/actioncli"
)

// RunActionCommand runs the public "run <action-id>" command line and returns its exit status. It never starts the
// app in this process: a running instance answers, or one is started in the background first.
func RunActionCommand(args []string) int {
	return actioncli.Main(args, actioncli.Deps{
		Program: cliProgram(), Dial: actionPlace().Dial, Start: startBackgroundInstance,
		Now: time.Now, Wait: time.Sleep, Stdout: os.Stdout, Stderr: os.Stderr,
	})
}

// actionPlace is the socket of this app identity: the data folder keeps development and installed builds apart.
func actionPlace() instance.Place {
	return instance.DefaultPlace(dataDir())
}

func startBackgroundInstance() error {
	exe, err := instance.Executable()
	if err != nil {
		return err
	}
	return instance.StartBackground(exe)
}

// cliProgram is how a user runs this executable: its bare name when that name on PATH is this very file, as for the
// installed package's command, else its full path.
func cliProgram() string {
	exe, err := instance.Executable()
	if err != nil {
		return appFolderSlug
	}
	base := filepath.Base(exe)
	if found, err := exec.LookPath(base); err == nil && sameFile(found, exe) {
		return base
	}
	return exe
}

func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// serveActionCommands answers the command line from this instance until quit. It runs once the palette and the
// shortcuts exist, so a request that arrives can run.
func (p *App) serveActionCommands() {
	ln, err := actionPlace().Listen()
	if err != nil {
		log.Println("action command line: not listening:", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	mygo.App.OnQuit(cancel)
	safego.Go("actioncli.serve", func(context.Context) { actioncli.Serve(ctx, ln, searchClock.Now, p.dispatchActionCommand) })
}

// dispatchActionCommand runs one requested action under a panic guard.
func (p *App) dispatchActionCommand(ctx context.Context, actionID string) actioncli.Response {
	var response actioncli.Response
	err := safego.Run("actioncli.dispatch", func(ctx context.Context) error {
		response = p.runActionCommand(ctx, actionID)
		return nil
	}, safego.WithContext(ctx))
	if err != nil {
		return actioncli.Response{Outcome: actioncli.Failed, Message: "the action crashed: " + err.Error()}
	}
	return response
}

// runActionCommand waits until the extensions have loaded, so an extension action is not rejected while it is still
// starting, then runs the action on the UI thread. An action the cached commands do not know yet is looked up again
// in the commands of every enabled extension, starting the ones that have not listed any. The listing does not depend
// on the palette having asked first, and it is used as listed: the manager's cache refuses it while its own background
// discovery of the same extension is still running, so reading the cache again would miss it.
func (p *App) runActionCommand(ctx context.Context, actionID string) actioncli.Response {
	select {
	case <-p.exts.settled.wait():
	case <-ctx.Done():
		return actioncli.Response{Outcome: actioncli.Failed, Message: "Nevermind is still starting; nothing ran"}
	}
	response := p.onUIAnswer(ctx, func() actioncli.Response { return p.runActionCommandOnUI(actionID) })
	if m := p.extensionManager(); response.Outcome == actioncli.Rejected && m != nil {
		table := globalShortcutTableOf(m.CommandsStarted(ctx))
		response = p.onUIAnswer(ctx, func() actioncli.Response {
			p.globalShortcuts = table
			return p.runActionCommandOnUI(actionID)
		})
	}
	return response
}

// errNoAnswer is the failure of a dispatch the UI thread did not take in time; the action may still run.
var errNoAnswer = errors.New("the running instance did not take the request in time; it may still run")

// onUIAnswer runs fn on the UI thread and returns its answer, or a failure when ctx ends first.
func (p *App) onUIAnswer(ctx context.Context, fn func() actioncli.Response) actioncli.Response {
	answer := make(chan actioncli.Response, 1)
	p.onUI(func() { answer <- fn() })
	select {
	case r := <-answer:
		return r
	case <-ctx.Done():
		return actioncli.Response{Outcome: actioncli.Failed, Message: errNoAnswer.Error()}
	}
}

// runActionCommandOnUI runs the action as its shortcut does and says what happened. Unknown, removed and disabled
// actions are rejected without running anything in their place.
func (p *App) runActionCommandOnUI(actionID string) actioncli.Response {
	switch actionID {
	case shortcuts.PaletteID:
		p.Show()
		return actioncli.Response{Outcome: actioncli.Opened}
	case shortcuts.PaletteToggleID:
		p.Toggle()
		return actioncli.Response{Outcome: actioncli.Opened}
	}
	if path, ok := shortcuts.AppPathOf(actionID); ok {
		return p.runAppActionCommand(path)
	}
	if entry, ok := p.extensionShortcutCommand(actionID); ok {
		if !p.extensionMayRun(entry.extID) {
			return actioncli.Response{Outcome: actioncli.Rejected, Message: "the extension of " + entry.command.Title + " is turned off"}
		}
		if runsWithoutPalette(entry.command) {
			p.runExtensionCommandInPlace(entry)
			return actioncli.Response{Outcome: actioncli.Started}
		}
	}
	if action, ok := p.shortcutAction(actionID); ok {
		title, _ := p.resolveShortcutTitle(actionID)
		p.Palette.OpenAction(title, action)
		return actioncli.Response{Outcome: actioncli.Opened}
	}
	return actioncli.Response{Outcome: actioncli.Rejected, Message: "no action has the ID " + actionID}
}

func (p *App) runAppActionCommand(path string) actioncli.Response {
	for _, a := range p.appsSnapshot() {
		if a.Path == path {
			if err := p.ports.Shell.LaunchApp(shellApp(a)); err != nil {
				return actioncli.Response{Outcome: actioncli.Failed, Message: err.Error()}
			}
			return actioncli.Response{Outcome: actioncli.Completed}
		}
	}
	return actioncli.Response{Outcome: actioncli.Rejected, Message: "the app " + path + " is not installed"}
}

// actionCommandLine is the command line that runs actionID as its shortcut does. The palette row's key toggles the
// palette, so its command line does too.
func actionCommandLine(actionID string) string {
	if actionID == shortcuts.PaletteID {
		actionID = shortcuts.PaletteToggleID
	}
	return actioncli.Invocation(cliProgram(), actionID, runtime.GOOS)
}

// copyActionCommandLine copies the command line of an action from the Keyboard Shortcuts list.
func (p *App) copyActionCommandLine(h search.Host, actionID string) {
	if err := p.ports.Clipboard.WriteText(actionCommandLine(actionID)); err != nil {
		h.Toast("Copy failed: " + err.Error())
		return
	}
	h.Toast("Command line copied")
}
