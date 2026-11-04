package system

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macSidebarPath is the System Settings sidebar, whose listed panes the root search offers on macOS.
const macSidebarPath = "/System/Applications/System Settings.app/Contents/Resources/Sidebar.plist"

// Options wires the services from the ports. Platform is the GOOS the panes and rows follow; Home is the user's
// home folder; SettingsTitle is the label of the settings command, which the OS chooses.
type Options struct {
	Ports         platform.Ports
	Platform      string
	Home          string
	SettingsTitle string
}

// Services runs system commands and reads what the palette shows about the system. PermissionRows
// answer from the capabilities and the ports without blocking; Run and RequestPermission may block and belong
// off the UI thread.
type Services struct {
	ports         platform.Ports
	goos          string
	home          string
	settingsTitle string

	panesOnce sync.Once
	panes     []Pane
}

// NewServices builds the services over the ports.
func NewServices(o Options) *Services {
	return &Services{ports: o.Ports, goos: o.Platform, home: o.Home, settingsTitle: o.SettingsTitle}
}

// Places lists the home folders the palette offers.
func (s *Services) Places() []Place {
	return Places(s.home)
}

// OpenPlace opens a folder with the OS file manager's default application.
func (s *Services) OpenPlace(place Place) error {
	return s.ports.Shell.OpenPath(place.Path)
}

// CommandTitle names a command for a failure sentence, with the OS label of the settings command.
func (s *Services) CommandTitle(id CommandID) string {
	return Title(id, s.settingsTitle)
}

// Run performs a command. Destructive commands must be confirmed by the extension that asks first; Run itself never asks.
func (s *Services) Run(id CommandID) error {
	system := s.ports.System
	switch id {
	case CommandLockScreen:
		return system.LockScreen()
	case CommandSleep:
		return system.Sleep()
	case CommandRestart:
		return system.Restart()
	case CommandShutDown:
		return system.ShutDown()
	case CommandLogOut:
		return system.LogOut()
	case CommandEmptyTrash:
		return system.EmptyTrash()
	case CommandSystemPanes:
		return system.OpenSettings("", "")
	}
	return platform.ErrUnsupported
}

// SettingsPanes returns the System Settings panes the root search offers. Only macOS has them; the sidebar is read
// once, on the first call, and an unreadable sidebar yields none.
func (s *Services) SettingsPanes() []Pane {
	if s.goos != "darwin" {
		return nil
	}
	s.panesOnce.Do(func() {
		sidebar, err := s.ports.FS.ReadFile(macSidebarPath)
		if err == nil {
			s.panes = PanesFromSidebar(string(sidebar))
		}
	})
	return s.panes
}

// OpenPane opens a settings pane, at an anchor section when the OS supports one.
func (s *Services) OpenPane(pane Pane, anchor string) error {
	return s.ports.System.OpenSettings(pane.ID, anchor)
}

// PermissionRows lists the permissions of this platform with their states.
func (s *Services) PermissionRows() []PermissionRow {
	return PermissionRows(s.goos, s.ports.Permissions)
}

// RequestPermission asks the OS for a permission. ErrUnsupported means the OS has no prompt, and the caller opens
// the settings pane instead.
func (s *Services) RequestPermission(kind platform.PermissionKind) error {
	return s.ports.Permissions.Request(kind)
}
