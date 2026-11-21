package platform

import "context"

// MaxHelperOutputBytes bounds each of stdout and stderr of a helper process. Output past the bound is
// discarded and HelperResult.Truncated is set, so a runaway tool cannot exhaust memory.
const MaxHelperOutputBytes = 1 << 20

// HelperResult is what a helper process wrote and how it ended.
type HelperResult struct {
	Stdout    []byte
	Stderr    []byte
	ExitCode  int
	Truncated bool
}

// Helper is the intent "run a small helper program the app ships or finds on the system and read what it
// prints". Every run has a deadline: the caller's context deadline, or the implementation's default. A
// helper that outlives its deadline is killed and Run returns the context error. A non-zero exit is not
// an error; it is ExitCode. Every OS supports it.
type Helper interface {
	// Run executes binary with args, feeding stdin when it is non-nil.
	Run(ctx context.Context, binary string, args []string, stdin []byte) (HelperResult, error)
}
