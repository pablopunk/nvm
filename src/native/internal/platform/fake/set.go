package fake

import "github.com/pablopunk/nvm/native/internal/platform"

// Set bundles one fake per port. Tests build a Set, hand Ports to the code under test, and keep the typed
// fields to script the world and assert on what the code did.
type Set struct {
	Clock        *Clock
	OSRelease    *OSRelease
	FS           *FS
	HTTP         *HTTP
	PublicHTTP   *HTTP
	Resolver     *Resolver
	Clipboard    *Clipboard
	Paster       *Paster
	SelectedText *SelectedText
	Shell        *Shell
	Screens      *Screens
	Notifier     *Notifier
	Keychain     *Keychain
	DeepLinks    *DeepLinks
	Processes    *Processes
	Helper       *Helper
	Commands     *Commands
	Hotkeys      *Hotkeys
	Focus        *Focus
	Tray         *TrayHost
	Autostart    *Autostart
	System       *System
	Permissions  *Permissions
	Windows      *WindowManager
	Desktops     *Desktops
	Microphone   *Microphone
	Opus         *OpusCodec
	FileDialog   *FileDialog
	Text         *TextRecognizer
	Capture      *ScreenCapturer
	OutputMute   *OutputMute
	FileDrag     *FileDrag
	Capabilities platform.Capabilities
}

// NewSet returns a complete set of empty fakes that supports every capability. Remove a capability from
// Capabilities to test the code path where the OS lacks it.
func NewSet() *Set {
	clipboard := NewClipboard()
	return &Set{
		Clock:        NewClock(),
		OSRelease:    NewOSRelease(),
		FS:           NewFS(),
		HTTP:         NewHTTP(),
		PublicHTTP:   NewHTTP(),
		Resolver:     NewResolver(),
		Clipboard:    clipboard,
		Paster:       NewPaster(),
		SelectedText: NewSelectedText(clipboard),
		Shell:        NewShell(),
		Screens:      NewScreens(),
		Notifier:     NewNotifier(),
		Keychain:     NewKeychain(),
		DeepLinks:    NewDeepLinks(),
		Processes:    NewProcesses(),
		Helper:       NewHelper(),
		Commands:     NewCommands(),
		Hotkeys:      NewHotkeys(),
		Focus:        NewFocus(),
		Tray:         NewTrayHost(),
		Autostart:    NewAutostart(),
		System:       NewSystem(),
		Permissions:  NewPermissions(),
		Windows:      NewWindowManager(),
		Desktops:     NewDesktops(),
		Microphone:   NewMicrophone(),
		Opus:         NewOpusCodec(),
		FileDialog:   NewFileDialog(),
		Text:         NewTextRecognizer(),
		Capture:      NewScreenCapturer(),
		OutputMute:   NewOutputMute(),
		FileDrag:     NewFileDrag(),
		Capabilities: platform.NewCapabilities(platform.AllCapabilities()...),
	}
}

// Ports returns the platform.Ports view of the set. The result shares state with the set, so changes made
// through the typed fields are seen by the code under test.
func (s *Set) Ports() platform.Ports {
	return platform.Ports{
		Clock:        s.Clock,
		OSRelease:    s.OSRelease,
		FS:           s.FS,
		HTTP:         s.HTTP,
		PublicHTTP:   s.PublicHTTP,
		Resolver:     s.Resolver,
		Clipboard:    s.Clipboard,
		Paster:       s.Paster,
		SelectedText: s.SelectedText,
		Shell:        s.Shell,
		Screens:      s.Screens,
		Notifier:     s.Notifier,
		Keychain:     s.Keychain,
		DeepLinks:    s.DeepLinks,
		Processes:    s.Processes,
		Helper:       s.Helper,
		Commands:     s.Commands,
		Hotkeys:      s.Hotkeys,
		Focus:        s.Focus,
		Tray:         s.Tray,
		Autostart:    s.Autostart,
		System:       s.System,
		Permissions:  s.Permissions,
		Windows:      s.Windows,
		Desktops:     s.Desktops,
		Microphone:   s.Microphone,
		Opus:         s.Opus,
		FileDialog:   s.FileDialog,
		Text:         s.Text,
		Capture:      s.Capture,
		OutputMute:   s.OutputMute,
		FileDrag:     s.FileDrag,
		Capabilities: s.Capabilities,
	}
}
