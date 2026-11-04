package fake

import "github.com/pablopunk/nvm/native/internal/platform"

// Set bundles one fake per port. Tests build a Set, hand Ports to the code under test, and keep the typed
// fields to script the world and assert on what the code did.
type Set struct {
	Clock        *Clock
	FS           *FS
	HTTP         *HTTP
	PublicHTTP   *HTTP
	Resolver     *Resolver
	Clipboard    *Clipboard
	Paster       *Paster
	Shell        *Shell
	Screens      *Screens
	Notifier     *Notifier
	Keychain     *Keychain
	DeepLinks    *DeepLinks
	Processes    *Processes
	Helper       *Helper
	Hotkeys      *Hotkeys
	Focus        *Focus
	Tray         *TrayHost
	Autostart    *Autostart
	System       *System
	Permissions  *Permissions
	Windows      *WindowManager
	Microphone   *Microphone
	Opus         *OpusCodec
	Capabilities platform.Capabilities
}

// NewSet returns a complete set of empty fakes that supports every capability. Remove a capability from
// Capabilities to test the code path where the OS lacks it.
func NewSet() *Set {
	return &Set{
		Clock:        NewClock(),
		FS:           NewFS(),
		HTTP:         NewHTTP(),
		PublicHTTP:   NewHTTP(),
		Resolver:     NewResolver(),
		Clipboard:    NewClipboard(),
		Paster:       NewPaster(),
		Shell:        NewShell(),
		Screens:      NewScreens(),
		Notifier:     NewNotifier(),
		Keychain:     NewKeychain(),
		DeepLinks:    NewDeepLinks(),
		Processes:    NewProcesses(),
		Helper:       NewHelper(),
		Hotkeys:      NewHotkeys(),
		Focus:        NewFocus(),
		Tray:         NewTrayHost(),
		Autostart:    NewAutostart(),
		System:       NewSystem(),
		Permissions:  NewPermissions(),
		Windows:      NewWindowManager(),
		Microphone:   NewMicrophone(),
		Opus:         NewOpusCodec(),
		Capabilities: platform.NewCapabilities(platform.AllCapabilities()...),
	}
}

// Ports returns the platform.Ports view of the set. The result shares state with the set, so changes made
// through the typed fields are seen by the code under test.
func (s *Set) Ports() platform.Ports {
	return platform.Ports{
		Clock:        s.Clock,
		FS:           s.FS,
		HTTP:         s.HTTP,
		PublicHTTP:   s.PublicHTTP,
		Resolver:     s.Resolver,
		Clipboard:    s.Clipboard,
		Paster:       s.Paster,
		Shell:        s.Shell,
		Screens:      s.Screens,
		Notifier:     s.Notifier,
		Keychain:     s.Keychain,
		DeepLinks:    s.DeepLinks,
		Processes:    s.Processes,
		Helper:       s.Helper,
		Hotkeys:      s.Hotkeys,
		Focus:        s.Focus,
		Tray:         s.Tray,
		Autostart:    s.Autostart,
		System:       s.System,
		Permissions:  s.Permissions,
		Windows:      s.Windows,
		Microphone:   s.Microphone,
		Opus:         s.Opus,
		Capabilities: s.Capabilities,
	}
}
