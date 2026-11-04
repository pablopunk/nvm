package platform

// Autostart is the intent "start the app when the user signs in". Enabling it writes the OS login item
// or autostart entry for the user only, never system-wide. Every OS supports it.
type Autostart interface {
	// Enabled reports whether the app starts at sign-in.
	Enabled() (bool, error)
	// Set turns the sign-in start on or off.
	Set(enabled bool) error
}
