// Package real implements the platform ports on the operating system: MyGo for the desktop intents
// (clipboard, shell, screens, notifications, secrets, tray, global shortcuts, login items) and the
// standard library for files, network, time, helper processes and per-OS process information.
//
// MyGo calls must run after the app has started (App.Run); the composition root builds these ports
// before that and the calls themselves fail loudly if made too early, as MyGo documents.
package real

import "github.com/pablopunk/nvm/native/internal/platform"

// NewPorts returns the production port set. It has no side effects until a port method is called.
func NewPorts() platform.Ports {
	clock := realClock{}
	return platform.Ports{
		Clock:        clock,
		FS:           osFS{},
		HTTP:         newHTTPClient(),
		Clipboard:    newClipboard(clock),
		Paster:       paster{},
		Shell:        shell{},
		Screens:      screens{},
		Notifier:     notifier{},
		Keychain:     keychain{},
		Processes:    processes{},
		Helper:       helper{},
		Hotkeys:      hotkeys{},
		Tray:         trayHost{},
		Autostart:    autostart{},
		Windows:      newWindowManager(helper{}),
		Capabilities: Capabilities(),
	}
}
