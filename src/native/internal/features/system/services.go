package system

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// macSidebarPath is the System Settings sidebar, whose listed panes the root search offers on macOS.
const macSidebarPath = "/System/Applications/System Settings.app/Contents/Resources/Sidebar.plist"

// Options wires the services from the ports. Platform is the GOOS the panes and rows follow; SettingsTitle is the
// label of the settings command, which the OS chooses.
type Options struct {
	Ports         platform.Ports
	Platform      string
	SettingsTitle string
}

// Services runs system commands and reads what the palette shows about the system. PermissionRows
// answer from the capabilities and the ports without blocking; Run and RequestPermission may block and belong
// off the UI thread.
type Services struct {
	ports         platform.Ports
	goos          string
	settingsTitle string

	panesOnce sync.Once
	panes     []Pane
}

// NewServices builds the services over the ports.
func NewServices(o Options) *Services {
	return &Services{ports: o.Ports, goos: o.Platform, settingsTitle: o.SettingsTitle}
}

// Places lists the home folders the palette offers, resolved under the home folder the system port reports.
func (s *Services) Places() []Place {
	return Places(s.ports.System.Home())
}

// CommandTitle names a command for a failure sentence, with the OS label of the settings command.
func (s *Services) CommandTitle(id CommandID) string {
	return Title(id, s.settingsTitle)
}

// Run performs a command. Destructive commands must be confirmed by the extension that asks first; Run itself never asks.
func (s *Services) Run(id CommandID) error {
	return s.RunContext(context.Background(), id)
}

// RunContext admits one native command; an admitted synchronous OS call may finish after cancellation.
func (s *Services) RunContext(ctx context.Context, id CommandID) error {
	if !s.Supported(id) {
		return platform.ErrUnsupported
	}
	binding := s.commandBinding(id)
	if err := ctx.Err(); err != nil {
		return err
	}
	return binding.run()
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
	if !s.Supported(CommandSystemPanes) {
		return platform.ErrUnsupported
	}
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
