package real

import "github.com/egoist/mygo"

// autostart toggles the user's sign-in start through MyGo's login item support: a login item on macOS, an
// XDG autostart entry on Linux and a Run entry on Windows.
type autostart struct{}

// Enabled reports whether the app starts at sign-in.
func (autostart) Enabled() (bool, error) {
	return mygo.App.OpenAtLogin(), nil
}

// Set turns the sign-in start on or off.
func (autostart) Set(enabled bool) error {
	return mygo.App.SetOpenAtLogin(enabled)
}
