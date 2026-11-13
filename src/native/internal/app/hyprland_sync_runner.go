package app

// syncRunner coalesces requests for the Hyprland sync and runs one sync at a time. Its running and again fields belong
// to the UI thread.
type syncRunner struct {
	coalescer
	running bool
	again   bool
}
