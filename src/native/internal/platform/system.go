package platform

// System is the intent "end the session or power state, open the system settings, empty the Trash, or quit the
// app". Each method is one user intent. A method whose intent the current system cannot fulfill returns
// ErrUnsupported; features omit those rows through the system.* capabilities before they are ever shown. Quit never
// returns ErrUnsupported: the extension API lists quit with the other system operations, so it is a method here too.
type System interface {
	// Quit asks the app to quit: its windows close, its stores flush and its menu bar items go, then the event loop
	// stops. It returns at once; the quit runs on the main thread.
	Quit()
	// Relaunch restarts the app through its normal quit lifecycle.
	Relaunch()
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
	// Home is the user's home folder, the root of the standard places (Downloads, Documents...). Empty when unknown,
	// so no place is ever resolved against a relative path.
	Home() string
}
