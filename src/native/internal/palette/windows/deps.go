// Package windows is the palette's window management: the commands the root lists, the window list that
// switches focus, and the settings that shape the layouts (WP4.1c). The engine and the catalog live below it in
// internal/windowmgmt; this package only turns them into palette rows and views.
package windows

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/commands"
)

// Deps are the ports the window views and commands use. The app wires them; the goldens and tests wire fakes.
type Deps struct {
	Engine   *windowmgmt.Service
	Settings *windowmgmt.SettingsStore
	// Target returns the app the palette was opened from, when the palette captured one.
	Target func() (windowmgmt.Target, bool)
	// OnUI runs fn on the UI thread. Views that change palette state from a background result use it.
	OnUI func(fn func())
	// Notify tells the user the outcome of a command that ran after the palette hid.
	Notify func(title, body string)
	// Go runs an engine call off the UI thread. The app runs it on a goroutine; tests run it in line.
	Go func(name string, fn func(ctx context.Context))
}

func (d Deps) runner() commands.Runner {
	return commands.Runner{Engine: d.Engine, Settings: d.Settings.Current}
}

// paletteInvocation is a command started from the palette: it acts on the app the palette was opened from.
func (d Deps) paletteInvocation() windowmgmt.Invocation {
	invocation := windowmgmt.Invocation{Source: windowmgmt.SourcePalette}
	if target, ok := d.Target(); ok {
		invocation.Target = &target
	}
	return invocation
}

// runInBackground runs one engine call off the UI thread and tells the user what it could not do. A call that
// reaches its window says nothing.
func (d Deps) runInBackground(name, title string, call func(ctx context.Context) string) {
	d.Go("windows."+name, func(ctx context.Context) {
		if message := call(ctx); message != "" {
			d.Notify(title, message)
		}
	})
}
