package platform

// Signal is a request a process can be sent. Interrupt and terminate ask it to stop; kill cannot be
// ignored.
type Signal string

// Signals the app sends.
const (
	SignalInterrupt Signal = "interrupt"
	SignalTerminate Signal = "terminate"
	SignalKill      Signal = "kill"
)

// Process describes one running process of the current user.
type Process struct {
	PID int
	// Name is the executable's short name, as the OS reports it.
	Name string
	// ResidentKB is the memory the process holds in RAM, in kilobytes.
	ResidentKB int64
}

// Processes is the intent "see what is running and stop it", for the Quit Apps command. Listing and
// signalling are capabilities processes.list and processes.signal, supported where the OS exposes
// per-process information to the app (Linux today; macOS and Windows return ErrUnsupported until
// implemented).
type Processes interface {
	// List returns the current user's processes, in no particular order.
	List() ([]Process, error)
	// Signal sends a request to a process. A missing process returns ErrNotFound.
	Signal(pid int, sig Signal) error
}
