package platform

// Hotkeys is the intent "run a command from anywhere, even while another app has focus", for the
// summon shortcut and the shortcuts the user assigns to commands. Accelerators use the canonical storage
// form, such as Command+Alt+K. Registering a shortcut another program holds returns ErrConflict. Every
// OS supports global shortcuts; Wayland sessions on Linux may refuse them and report ErrUnsupported.
type Hotkeys interface {
	// Register binds accelerator to fn, which runs on a goroutine of the platform's choosing.
	Register(accelerator string, fn func()) error
	// Unregister removes the binding. Removing an unbound accelerator does nothing.
	Unregister(accelerator string)
}
