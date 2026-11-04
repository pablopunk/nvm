package platform

// System is the intent "end the session or power state, open the system settings, or empty the Trash". Each
// method is one user intent. A method whose intent the current system cannot fulfill returns ErrUnsupported;
// features omit those rows through the system.* capabilities before they are ever shown.
type System interface {
	// LockScreen locks the computer and asks for the user's password to continue.
	LockScreen() error
	// Sleep puts the computer to sleep.
	Sleep() error
	// Restart restarts the computer. Apps are asked to quit by the OS, as it does for a normal restart.
	Restart() error
	// ShutDown powers the computer off.
	ShutDown() error
	// LogOut ends the user's session.
	LogOut() error
	// EmptyTrash permanently deletes everything in the user's Trash. It is irreversible; callers confirm first.
	EmptyTrash() error
	// OpenSettings opens the system settings. pane and anchor name a settings pane and a section within it on
	// systems that can open one directly (macOS); other systems ignore them and open the top level.
	OpenSettings(pane, anchor string) error
}
