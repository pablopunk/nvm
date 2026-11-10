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
	env := processSystemEnvironment()
	commandPort := commands{userPath: newUserPathCache()}
	return platform.Ports{
		OSRelease:       newOSRelease(),
		Clock:           realClock{},
		FS:              osFS{},
		HTTP:            newHTTPClient(),
		PublicHTTP:      newPublicHTTP(),
		Resolver:        resolver{},
		Clipboard:       newClipboard(),
		Paster:          NewPaster(helper{}, nil),
		SelectedText:    NewSelectedText(helper{}),
		Shell:           shell{},
		Screens:         screens{},
		Notifier:        notifier{},
		Keychain:        keychain{},
		DeepLinks:       deepLinks{},
		Processes:       processes{},
		Helper:          helper{},
		Commands:        commandPort,
		Desktops:        NewDesktopSwitcher(env.goos, commandPort),
		Hotkeys:         hotkeys{},
		Focus:           newFocus(),
		Tray:            trayHost{},
		Autostart:       autostart{},
		System:          newSystem(helper{}, shell{}, env.goos, env.home, env.user),
		Permissions:     newPermissions(helper{}),
		Windows:         newWindowManager(helper{}),
		Capabilities:    Capabilities(),
		Microphone:      newMicrophone(),
		Opus:            opusCodec{},
		FileDialog:      fileDialog{},
		Text:            newTextRecognizer(helper{}),
		FilePictures:    newFilePictures(helper{}),
		Capture:         newScreenCapturer(helper{}),
		OutputMute:      newOutputMute(helper{}),
		FileDrag:        fileDrag{},
		DirectoryAccess: directoryAccess{},
	}
}
