package platform

import "context"

// CommandSpec is one process an extension asked the host to run. Dir is the working folder ("" for the host's
// own). Env entries are added to the environment, and the user's login PATH is put in front of the host's PATH,
// so tools installed by the user's shell profile are found from an app that was launched from the Dock.
type CommandSpec struct {
	Binary string
	Args   []string
	Dir    string
	Env    map[string]string
}

// Commands is the intent "run a program the user asked an extension to run and read what it prints". The caller
// sets the deadline on ctx; a process still running at the deadline is killed and Run returns the context error.
// A non-zero exit is not an error; it is ExitCode. A program that cannot start is an error. Output is capped at
// MaxHelperOutputBytes per stream, as Helper caps it.
type Commands interface {
	Run(ctx context.Context, spec CommandSpec) (HelperResult, error)
}
