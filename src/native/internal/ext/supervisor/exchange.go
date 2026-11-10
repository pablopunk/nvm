package supervisor

import (
	"context"
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
				_, _ = proc.Wait()
				return
			}
		}
	}()
	return out
}

// exchange admits a request to one child and waits for its response within the method's budget. Cancellation before
// admission prevents dispatch; an ordinary request already dispatched may finish. Stop cancels its captured lifetime.
func (r *Runtime) exchange(admission, ctx, lifetime context.Context, gate *dispatchGate, req protocol.Request, onDenied func(*api.SnapshotDenial)) (protocol.Response, error) {
	child, budget, err := r.childForCall(admission, lifetime, req.Method)
	if err != nil {
		return protocol.Response{}, err
	}
	line, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "request cannot be encoded")
	}
	if err := gate.admit(admission, lifetime); err != nil {
		return protocol.Response{}, err
	}
	if err := child.proc.WriteLine(line); err != nil {
		return protocol.Response{}, r.failChild(child, api.CodeInternal, "cannot send a request: "+err.Error())
	}
	return r.awaitResponse(ctx, lifetime, child, req, budget, onDenied)
}

// childForCall binds a request to a particular child after any in-progress startup. Stop ends the captured lifetime,
// so a request cannot follow an installation across disable, replacement, or an explicit restart.
func (r *Runtime) childForCall(ctx, lifetime context.Context, method string) (*childProcess, Budget, error) {
	waitStarting := method == protocol.MethodRun || method == protocol.MethodLoad || method == protocol.MethodSearch
	for {
		if err := ctx.Err(); err != nil {
			return nil, Budget{}, err
		}
		r.mu.Lock()
		if r.restartingSearch {
			waitStarting = true
		}
		if r.lifetime != lifetime || lifetime.Err() != nil {
			r.mu.Unlock()
			return nil, Budget{}, unavailable("stopped")
		}
		if r.state == StateRunning && r.child != nil {
			child, budget := r.child, r.cfg.BudgetFor(method)
			r.mu.Unlock()
			return child, budget, nil
		}
		if r.state == StateDormant {
			r.startLocked()
			waitStarting = true
		}
		if r.state == StateFailed && !r.cfg.Clock.Now().Before(r.retryAt) {
			r.startLocked()
		}
		if r.state != StateStarting || !waitStarting {
			reason := notRunningReason(r.state, r.reason)
			r.mu.Unlock()
			return nil, Budget{}, unavailable(reason)
		}
		changed := r.changed
		r.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, Budget{}, ctx.Err()
		case <-lifetime.Done():
			return nil, Budget{}, unavailable("stopped")
		}
	}
}

// notRunningReason names the state of a runtime that cannot take a call, with the reason it recorded. The
// reason is the one a palette row shows, so it must not be replaced by the bare state name.
func notRunningReason(state State, reason string) string {
	switch state {
	case StateIdle, StateDormant:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateFailed:
		return "restarting after a failure: " + reason
	case StateBroken:
		return "does not load: " + reason
	case StateDisabled:
		return "disabled: " + reason
	}
	if reason == "" {
		return "not running: " + string(state)
	}
	return reason
}

// awaitResponse waits for the child's answer to req. The budgets count the child's own time only: the time the host
// spends answering a service call is added back to both the soft and the hard deadline, because an extension that
// waits for a shell command, a network reply or a palette confirmation is not busy. The soft budget only records a
// diagnostic; the hard budget kills the child.
func (r *Runtime) awaitResponse(ctx, lifetime context.Context, child *childProcess, req protocol.Request, budget Budget, onDenied func(*api.SnapshotDenial)) (protocol.Response, error) {
	clock := r.cfg.Clock
	started := clock.Now()
	var blocked time.Duration
	soft := r.softTimer(budget)
	hard := clock.After(budget.Hard)
	var chunks []byte
	sequence := 0
	var permissionFailure *api.Failure
	evidence := &api.SnapshotEvidence{}
	deniedAuthorityNotified := false
	deniedPathsNotified := make(map[string]bool)
	for {
		select {
		case <-lifetime.Done():
			return protocol.Response{}, unavailable("stopped")
		case <-ctx.Done():
			return protocol.Response{}, r.cancelRequestChild(child, req.Method, ctx.Err())
		case line, open := <-child.lines:
			if call, ok := serviceCallFrom(line, open); ok {
				if sequence != 0 {
					return protocol.Response{}, r.failChild(child, api.CodeInvalid, "service call interrupted response chunks")
				}
				callStart := clock.Now()
				var reply protocol.ServiceReply
				var err error
				if req.Method == protocol.MethodSearch || req.Invocation != nil && req.Invocation.AwaitInitialView {
					reply, err = r.answerViewService(ctx, lifetime, child, call, req.Method)
					if ctx.Err() != nil {
						return protocol.Response{}, r.cancelRequestChild(child, req.Method, ctx.Err())
					}
					if errors.Is(err, context.DeadlineExceeded) {
						return protocol.Response{}, r.failChild(child, api.CodeTimeout, "view service did not finish in time")
					}
					if err != nil {
						return protocol.Response{}, err
					}
				} else {
					reply, err = r.answerViewService(context.Background(), lifetime, child, call, req.Method)
					if errors.Is(err, context.DeadlineExceeded) {
						return protocol.Response{}, r.failChild(child, api.CodeTimeout, "service did not finish in time")
					}
				}
				denial := observeSnapshotService(evidence, call, reply)
				if denial != nil {
					if permissionFailure == nil {
						permissionFailure = denial.Failure
					}
					notify := !denial.AuthorityDenied && !deniedPathsNotified[denial.Path] || denial.AuthorityDenied && !deniedAuthorityNotified
					if denial.AuthorityDenied {
						deniedAuthorityNotified = true
					} else if len(deniedPathsNotified) < 200 {
						deniedPathsNotified[denial.Path] = true
					} else {
						notify = false
					}
					if onDenied != nil && notify {
						onDenied(denial)
					}
				}
				blocked += clock.Now().Sub(callStart)
				if err != nil {
					return protocol.Response{}, err
				}
				computed := clock.Now().Sub(started) - blocked
				hard = clock.After(remainingBudget(budget.Hard, computed))
				if soft != nil {
					soft = clock.After(remainingBudget(budget.Soft, computed))
				}
				continue
			}
			if open && line.err == nil {
				var probe struct {
					Chunk *json.RawMessage `json:"chunk"`
				}
				if json.Unmarshal(line.data, &probe) == nil && probe.Chunk != nil {
					if req.Method != protocol.MethodRun {
						return protocol.Response{}, r.failChild(child, api.CodeInvalid, "response chunks are not allowed for this method")
					}
					var part protocol.ResponseChunk
					if json.Unmarshal(line.data, &part) != nil || part.ID != req.ID || part.Sequence != sequence || len(part.Chunk) == 0 || len(part.Chunk) > 256<<10 || len(chunks)+len(part.Chunk) > protocol.MaxResponseBytes {
						return protocol.Response{}, r.failChild(child, api.CodeInvalid, "invalid response chunk")
					}
					chunks = append(chunks, part.Chunk...)
					sequence++
					if !part.Last {
						continue
					}
					line.data = chunks
				} else if sequence != 0 {
					return protocol.Response{}, r.failChild(child, api.CodeInvalid, "incomplete response chunks")
				}
			}
			response, err := r.handleLine(child, req, line, open)
			response.PermissionFailure = permissionFailure
			response.SnapshotEvidence = evidence
			return response, err
		case <-soft:
			soft = nil
			r.recordSlow(req.Method, budget)
		case <-hard:
			return protocol.Response{}, r.failChild(child, api.CodeTimeout, fmt.Sprintf("%s did not finish within %s", req.Method, budget.Hard))
		}
	}
}

// remainingBudget is what is left of a budget after computed time of child work. It is never negative.
func remainingBudget(budget, computed time.Duration) time.Duration {
	if remaining := budget - computed; remaining > 0 {
		return remaining
	}
	return 0
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

// cancelViewChild abandons a canceled view request without counting it as an extension failure.
func (r *Runtime) cancelViewChild(child *childProcess, err error) error {
	r.mu.Lock()
	if r.child == child {
		r.detachChildLocked()
		r.state = StateIdle
		r.startLocked()
	}
	r.mu.Unlock()
	_ = child.proc.Kill()
	return err
}
