package platform

// MaxWorkerStderrBytes bounds the stderr text kept from a worker. Only the last bytes are kept, because a
// crash message is printed last.
const MaxWorkerStderrBytes = 8 << 10

// WorkerSpec describes a long-lived helper program that the app starts and talks to over its standard
// streams. The helper is a child process: it cannot take the app down or block it.
type WorkerSpec struct {
	Binary string
	Args   []string
	// MaxLineBytes bounds one line the worker writes on stdout. A longer line fails ReadLine.
	MaxLineBytes int
}

// Worker is the intent "run a helper program for as long as its owner needs it, exchange line-delimited
// messages with it, and stop it when it misbehaves". Every OS supports it. Memory limits are the
// helper's own business: the helper applies them to itself at start.
type Worker interface {
	// Start launches the helper. It returns once the process exists; the helper's own readiness is a
	// message the owner reads.
	Start(spec WorkerSpec) (WorkerProcess, error)
}

// WorkerProcess is one running helper. WriteLine and ReadLine may be called from different goroutines;
// concurrent calls to WriteLine, or to ReadLine, must be serialized by the owner.
type WorkerProcess interface {
	// PID identifies the process for process queries such as memory watches.
	PID() int
	// WriteLine sends one line on the helper's stdin. The newline is added.
	WriteLine(line []byte) error
	// ReadLine blocks until the helper writes one line on stdout, without its newline. It returns an
	// error once the helper has exited or the line exceeded MaxLineBytes.
	ReadLine() ([]byte, error)
	// Kill stops the helper at once. It is safe to call more than once and after exit.
	Kill() error
	// Wait reaps the helper and returns its exit code. The owner calls it once ReadLine has failed.
	Wait() (exitCode int, err error)
	// StderrTail returns the last bytes the helper wrote on stderr, at most MaxWorkerStderrBytes.
	StderrTail() string
}
