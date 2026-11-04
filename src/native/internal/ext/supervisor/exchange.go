package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// childProcess is one running child and the reader that turns its stdout into lines.
type childProcess struct {
	proc       platform.WorkerProcess
	lines      <-chan lineResult
	done       chan struct{}
	generation uint64
	// methods are the protocol methods the child's source declared in its ready line.
	methods []string
}

// lineResult is one line from the child, or the final result when the child ends: the error that stopped
// reading, the exit code and the stderr tail.
type lineResult struct {
	data     []byte
	err      error
	exitCode int
	stderr   string
}

// readLines forwards each line of the child's stdout until the child ends. The reader reaps the process once
// its stdout ends, and stops early when done is closed.
func readLines(proc platform.WorkerProcess, done <-chan struct{}) <-chan lineResult {
	out := make(chan lineResult)
	go func() {
		defer close(out)
		for {
			line, err := proc.ReadLine()
			if err != nil {
				if errors.Is(err, platform.ErrLineTooLong) {
					_ = proc.Kill()
				}
				code, _ := proc.Wait()
				select {
				case out <- lineResult{err: err, exitCode: code, stderr: proc.StderrTail()}:
				case <-done:
				}
				return
			}
			select {
			case out <- lineResult{data: line}:
			case <-done:
				return
			}
		}
	}()
	return out
}

// exchange writes one request to the running child and waits for its response, within the method's budget.
// Any host-side failure kills the child and is recorded, so the next call starts from a clean state.
func (r *Runtime) exchange(req protocol.Request) (protocol.Response, error) {
	child, budget, err := r.runningChild(req.Method)
	if err != nil {
		return protocol.Response{}, err
	}
	line, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "request cannot be encoded")
	}
	if err := child.proc.WriteLine(line); err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "cannot send a request: "+err.Error())
	}
	return r.awaitResponse(child, req, budget)
}

// runningChild returns the child a request should go to, with the budget of its method.
func (r *Runtime) runningChild(method string) (*childProcess, Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != StateRunning || r.child == nil {
		return nil, Budget{}, unavailable(notRunningReason(r.state, r.reason))
	}
	return r.child, r.cfg.BudgetFor(method), nil
}

// notRunningReason names the state of a runtime that cannot take a call, with the reason it recorded. The
// reason is the one a palette row shows, so it must not be replaced by the bare state name.
func notRunningReason(state State, reason string) string {
	if reason == "" {
		return "not running: " + string(state)
	}
	return reason
}

// awaitResponse waits for the child's answer to req. The soft budget only records a diagnostic; the hard budget
// kills the child.
func (r *Runtime) awaitResponse(child *childProcess, req protocol.Request, budget Budget) (protocol.Response, error) {
	soft := r.softTimer(budget)
	hard := r.cfg.Clock.After(budget.Hard)
	for {
		select {
		case line, open := <-child.lines:
			if call, ok := serviceCallFrom(line, open); ok {
				if err := r.answerService(child, call, budget); err != nil {
					return protocol.Response{}, err
				}
				continue
			}
			return r.handleLine(child, req, line, open)
		case <-soft:
			soft = nil
			r.recordSlow(req.Method, budget)
		case <-hard:
			return protocol.Response{}, r.failChild(child, api.CodeTimeout, fmt.Sprintf("%s did not finish within %s", req.Method, budget.Hard))
		}
	}
}

func (r *Runtime) softTimer(budget Budget) <-chan time.Time {
	if budget.Soft <= 0 {
		return nil
	}
	return r.cfg.Clock.After(budget.Soft)
}

func (r *Runtime) recordSlow(method string, budget Budget) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addDiagnosticLocked(fmt.Sprintf("%s took longer than %s", method, budget.Soft))
}

// handleLine turns one line from the child into a response, or into a failure that ends the child.
func (r *Runtime) handleLine(child *childProcess, req protocol.Request, line lineResult, open bool) (protocol.Response, error) {
	if !open {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "extension process ended unexpectedly")
	}
	if line.err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, describeExit(line))
	}
	var response protocol.Response
	if err := json.Unmarshal(line.data, &response); err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "response is not valid JSON")
	}
	if response.ID != req.ID {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "response answered another request")
	}
	return response, nil
}

// describeExit explains why a child stopped reading: its exit code and the last line it wrote on stderr.
func describeExit(line lineResult) string {
	if errors.Is(line.err, platform.ErrLineTooLong) {
		return "extension output exceeded the protocol line limit"
	}
	reason := fmt.Sprintf("extension process exited with code %d", line.exitCode)
	if last := lastLine(line.stderr); last != "" {
		reason += ": " + last
	}
	return reason
}

// lastLine returns the line that best explains a stderr tail, trimmed to a palette-safe length. A Go panic or
// fatal error names its cause on one line that the stack trace follows, so that line wins over the trace.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
			return clipLine(strings.TrimSpace(line))
		}
	}
	return clipLine(strings.TrimSpace(lines[len(lines)-1]))
}

func clipLine(line string) string {
	if len(line) > 200 {
		return line[:200] + "..."
	}
	return line
}

// failChild ends child, records the failure and returns the error the caller sees. A child that is no longer
// the current one (it was stopped or replaced meanwhile) only has its process killed.
func (r *Runtime) failChild(child *childProcess, code api.ErrorCode, reason string) error {
	r.mu.Lock()
	current := r.child == child
	if current {
		r.child = nil
		close(child.done)
		r.recordFailureLocked(reason)
	}
	r.mu.Unlock()
	_ = child.proc.Kill()
	return &CallError{Failure: api.Failure{Code: code, Message: reason}}
}
