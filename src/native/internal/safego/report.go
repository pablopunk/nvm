package safego

import (
	"errors"
	"fmt"
	"os"
)

// ErrDisabled is wrapped by the error of a call whose restart policy gave up after repeated failures.
var ErrDisabled = errors.New("safego: disabled after repeated failures")

// Failure describes one failed attempt as handed to a Reporter.
type Failure struct {
	Name     string // the name the call was started with
	Panic    bool   // true when the attempt panicked; Stack is set
	Err      error  // the panic as an error, the returned error, or the timeout
	Stack    string // goroutine stack captured at the panic, empty otherwise
	Attempt  int    // failures counted within the restart window, starting at 1
	Disabled bool   // true for the report that disables the call
}

// Reporter receives every failure of a call. Report returns the support reference it recorded for the
// failure, or "" when it keeps none. A panic's PanicError carries the reference returned for it.
type Reporter interface {
	Report(f Failure) string
}

// ReporterFunc adapts a plain function to Reporter.
type ReporterFunc func(f Failure) string

// Report calls fn.
func (fn ReporterFunc) Report(f Failure) string { return fn(f) }

// PanicError is the error a call returns when its function panicked.
type PanicError struct {
	Name  string
	Value any
	Stack string
	Ref   string
}

// Error names the call and the panic value. The stack stays in Stack for reporters.
func (e *PanicError) Error() string {
	return fmt.Sprintf("safego: %s panicked: %v", e.Name, e.Value)
}

// stderrReporter is the fallback when no reporter is installed, so a failure is never silent.
type stderrReporter struct{}

// Report writes the failure to standard error and keeps no reference.
func (stderrReporter) Report(f Failure) string {
	fmt.Fprintf(os.Stderr, "safego: %s failed (attempt %d, panic %t, disabled %t): %v\n",
		f.Name, f.Attempt, f.Panic, f.Disabled, f.Err)
	return ""
}
