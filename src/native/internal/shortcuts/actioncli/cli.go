package actioncli

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// RunVerb is the command line's verb: "<program> run <action-id>".
const RunVerb = "run"

// The exit statuses of the command line.
const (
	// ExitOK: the action completed, started, or opened the palette.
	ExitOK = 0
	// ExitUsage: the command line was malformed.
	ExitUsage = 2
	// ExitRejected: no registered action has the ID, or it is unavailable; nothing ran.
	ExitRejected = 3
	// ExitFailed: the action was found but failed.
	ExitFailed = 4
	// ExitUnavailable: no instance could be reached or started in time, or the result was lost.
	ExitUnavailable = 5
)

const (
	// ReadinessTimeout bounds how long a cold start may take to answer.
	ReadinessTimeout = 15 * time.Second
	pollInterval     = 100 * time.Millisecond
)

// Deps is what the command line needs: the program name for help, the socket, a background start, a clock and output.
type Deps struct {
	Program string
	Dial    Dial
	// Start launches one instance in the background. It is called at most once per invocation.
	Start  func() error
	Now    func() time.Time
	Wait   func(time.Duration)
	Stdout io.Writer
	Stderr io.Writer
}

// IsCommand reports whether args (without the program name) are this command line's, so the entry point routes them
// here instead of starting the app.
func IsCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case RunVerb, "help", "--help", "-h":
		return true
	}
	return false
}

// Usage is the help text.
func Usage(program string) string {
	return fmt.Sprintf(`Usage: %[1]s run <action-id>

Runs one Nevermind action, as its shortcut would. If Nevermind is running, the
request goes to it; otherwise Nevermind starts in the background first. The
palette opens only for actions that need it.

Copy an action's exact command from Command-K: open Keyboard Shortcuts, select
the action and choose "Copy Command Line".

Exit status:
  %[2]d  the action completed, started, or opened the palette
  %[3]d  the command line is malformed
  %[4]d  no action has that ID, or it is unavailable; nothing ran
  %[5]d  the action failed
  %[6]d  Nevermind could not be reached or started, or the result was lost
`, program, ExitOK, ExitUsage, ExitRejected, ExitFailed, ExitUnavailable)
}

// Main runs the command line and returns its exit status.
func Main(args []string, deps Deps) int {
	if len(args) == 0 || args[0] != RunVerb {
		fmt.Fprint(deps.Stdout, Usage(deps.Program))
		return ExitOK
	}
	if len(args) != 2 {
		fmt.Fprint(deps.Stderr, Usage(deps.Program))
		return ExitUsage
	}
	actionID := args[1]
	if err := ValidateActionID(actionID); err != nil {
		fmt.Fprintf(deps.Stderr, "%s: %v\n", deps.Program, err)
		return ExitUsage
	}
	response, err := forwardOrStart(actionID, deps)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "%s: %v\n", deps.Program, err)
		return ExitUnavailable
	}
	return report(response, actionID, deps)
}

// forwardOrStart forwards to a running instance, or starts one and forwards once it answers. The request is written at
// most once, so a slow or lost answer never runs the action twice.
func forwardOrStart(actionID string, deps Deps) (Response, error) {
	response, err := Forward(deps.Dial, actionID)
	if !errors.Is(err, ErrNoInstance) {
		return response, err
	}
	if err := deps.Start(); err != nil {
		return Response{}, fmt.Errorf("could not start Nevermind: %w", err)
	}
	deadline := deps.Now().Add(ReadinessTimeout)
	for {
		response, err = Forward(deps.Dial, actionID)
		if !errors.Is(err, ErrNoInstance) {
			return response, err
		}
		if !deps.Now().Before(deadline) {
			return Response{}, fmt.Errorf("the started instance did not answer within %s", ReadinessTimeout)
		}
		deps.Wait(pollInterval)
	}
}

func report(r Response, actionID string, deps Deps) int {
	message := r.Message
	switch r.Outcome {
	case Completed, Started, Opened:
		if message != "" {
			fmt.Fprintln(deps.Stdout, message)
		}
		return ExitOK
	case Rejected:
		if message == "" {
			message = fmt.Sprintf("no action has the ID %q", actionID)
		}
		fmt.Fprintf(deps.Stderr, "%s: %s\n", deps.Program, message)
		return ExitRejected
	default:
		if message == "" {
			message = "the action failed"
		}
		fmt.Fprintf(deps.Stderr, "%s: %s\n", deps.Program, message)
		return ExitFailed
	}
}
