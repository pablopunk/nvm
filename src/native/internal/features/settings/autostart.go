package settings

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// LaunchAtLogin is the sign-in start of the app. It belongs to the user, never the system, and is offered only when
// the OS supports the autostart capability.
type LaunchAtLogin struct {
	port      platform.Autostart
	supported bool
}

// NewLaunchAtLogin wraps the Autostart port. supported is the autostart capability of the system.
func NewLaunchAtLogin(port platform.Autostart, supported bool) LaunchAtLogin {
	return LaunchAtLogin{port: port, supported: supported}
}

// Supported reports whether the settings should offer the toggle at all.
func (l LaunchAtLogin) Supported() bool {
	return l.supported && l.port != nil
}

// Enabled reports whether the app starts at sign-in. An unreadable state reads as off, never as on.
func (l LaunchAtLogin) Enabled() bool {
	if !l.Supported() {
		return false
	}
	on, err := l.port.Enabled()
	return err == nil && on
}

// Set turns the sign-in start on or off. It returns the sentence the palette shows: what happened, or why not.
func (l LaunchAtLogin) Set(on bool) (string, error) {
	if !l.Supported() {
		return "Start at login is not available on this system", platform.ErrUnsupported
	}
	if err := l.port.Set(on); err != nil {
		return fmt.Sprintf("Could not %s start at login", verb(on)), err
	}
	if on {
		return "Nevermind will start at login", nil
	}
	return "Nevermind will not start at login", nil
}

// verb is the infinitive of the change, for the failure sentence.
func verb(on bool) string {
	if on {
		return "enable"
	}
	return "disable"
}
