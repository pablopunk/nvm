package supervisor

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// startLocked launches a child in the background unless one is running, starting, broken, disabled, or failed
// and still inside its backoff. The caller holds r.mu.
func (r *Runtime) startLocked() {
	switch r.state {
	case StateRunning, StateStarting, StateBroken, StateDisabled:
		return
	case StateFailed:
		if r.cfg.Clock.Now().Before(r.retryAt) {
			return
		}
	}
	r.state = StateStarting
	r.generation++
	if r.restartingSearch {
		r.wakeLocked()
	} else {
		r.notifyLocked()
	}
	go r.launch(r.generation)
}

// launch starts the helper process for one generation and waits for its ready line. The generation check on
// every transition keeps a child from being adopted after Stop or a newer start.
func (r *Runtime) launch(generation uint64) {
	done := make(chan struct{})
	proc, err := r.cfg.Worker.Start(platform.WorkerSpec{
		Binary:       r.cfg.Executable,
		Args:         r.childArgs(),
		MaxLineBytes: protocol.MaxLineBytes,
	})
	if err != nil {
		r.startFailed(generation, "cannot start the extension process: "+err.Error(), false)
		return
	}
	child := &childProcess{proc: proc, done: done, generation: generation}
	child.lines = readLines(proc, done)
	ready, broken, reason := r.awaitReady(child)
	if !ready {
		close(done)
		_ = proc.Kill()
		r.startFailed(generation, reason, broken)
		return
	}
	r.adopt(generation, child)
}

// awaitReady reads the child's first line within the start timeout. It reports whether the child loaded, and
// when it did not, whether the source is broken (not worth retrying) or the child failed (retry with backoff).
func (r *Runtime) awaitReady(child *childProcess) (ready bool, broken bool, reason string) {
	select {
	case line, open := <-child.lines:
		if !open || line.err != nil {
			return false, false, describeOrEnded(line, open)
		}
		var message protocol.Ready
		if err := json.Unmarshal(line.data, &message); err != nil {
			return false, false, "the ready line is not valid JSON"
		}
		if !message.OK {
			return false, true, failureMessage(message.Failure)
		}
		child.methods = message.Methods
		return true, false, ""
	case <-r.cfg.Clock.After(r.cfg.startTimeout()):
		return false, false, fmt.Sprintf("did not start within %s", r.cfg.startTimeout())
	}
}

func describeOrEnded(line lineResult, open bool) string {
	if !open {
		return "the extension process ended before it was ready"
	}
	return describeExit(line)
}

func failureMessage(failure *api.Failure) string {
	if failure == nil || failure.Message == "" {
		return "the extension source does not load"
	}
	return failure.Message
}

// adopt makes a ready child the running one, unless the generation changed while it was starting.
func (r *Runtime) adopt(generation uint64, child *childProcess) {
	r.mu.Lock()
	if generation != r.generation || r.state != StateStarting {
		r.mu.Unlock()
		close(child.done)
		_ = child.proc.Kill()
		return
	}
	quiet := r.restartingSearch && sameMethods(r.declared, child.methods)
	r.child = child
	r.state = StateRunning
	r.restartingSearch = false
	r.reason = ""
	if quiet {
		r.wakeLocked()
	} else {
		r.notifyLocked()
	}
	r.mu.Unlock()
	r.logLifecycle(logx.Info, "extension running", nil)
	go r.watchResident(child)
}

func sameMethods(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// logLifecycle writes one lifecycle line for the extension, with its reason when it has one.
func (r *Runtime) logLifecycle(level logx.Level, msg string, fields logx.F) {
	if r.cfg.Log == nil {
		return
	}
	if fields == nil {
		fields = logx.F{}
	}
	fields["extension"] = r.identity.ID
	if level == logx.Warn {
		r.cfg.Log.Warn(msg, fields)
		return
	}
	r.cfg.Log.Info(msg, fields)
}

// startFailed records a child that never became ready. A broken source stops retries; anything else counts as
// a failure with backoff.
func (r *Runtime) startFailed(generation uint64, reason string, broken bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation != r.generation {
		return
	}
	r.restartingSearch = false
	if broken {
		r.state = StateBroken
		r.reason = reason
		r.addDiagnosticLocked("does not load: " + reason)
		r.notifyLocked()
		r.logLifecycle(logx.Warn, "extension does not load", logx.F{"reason": reason})
		return
	}
	r.recordFailureLocked(reason)
}

// detachChildLocked removes the running child from the runtime and wakes its reader. The caller kills it.
func (r *Runtime) detachChildLocked() *childProcess {
	child := r.child
	if child != nil {
		r.child = nil
		close(child.done)
	}
	return child
}

// recordFailureLocked counts a failure inside the failure window. Enough of them disable the extension; fewer
// schedule the next start after an exponential backoff.
func (r *Runtime) recordFailureLocked(reason string) {
	now := r.cfg.Clock.Now()
	window := r.cfg.failureWindow()
	kept := r.failures[:0]
	for _, at := range r.failures {
		if now.Sub(at) < window {
			kept = append(kept, at)
		}
	}
	r.failures = append(kept, now)
	r.reason = reason
	r.addDiagnosticLocked("failed: " + reason)
	defer r.notifyLocked()
	if len(r.failures) >= r.cfg.maxFailures() {
		r.state = StateDisabled
		r.reason = fmt.Sprintf("disabled after %d failures in %s; last: %s", len(r.failures), window, reason)
		r.logLifecycle(logx.Warn, "extension disabled after repeated failures", logx.F{"reason": r.reason})
		return
	}
	r.state = StateFailed
	r.retryAt = now.Add(backoffFor(len(r.failures)))
	r.logLifecycle(logx.Warn, "extension failed; restart after backoff", logx.F{"reason": reason})
}

// backoffFor is the wait after the n-th failure: 250 ms, doubling, capped at 30 s.
func backoffFor(n int) time.Duration {
	delay := backoffBase
	for i := 1; i < n && delay < backoffMax; i++ {
		delay *= 2
	}
	if delay > backoffMax {
		return backoffMax
	}
	return delay
}

// childArgs are the arguments that make this binary serve as the extension child.
func (r *Runtime) childArgs() []string {
	identity, _ := json.Marshal(r.identity)
	return []string{
		protocol.ChildFlag,
		r.sourcePath,
		strconv.FormatUint(r.cfg.MemoryLimit, 10),
		string(identity),
	}
}
