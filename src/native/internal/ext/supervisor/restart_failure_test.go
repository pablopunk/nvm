package supervisor

import (
	"strings"
	"testing"
)

func TestSearchDuringARestartAfterAFailureAnswersWithTheFailure(t *testing.T) {
	requireUnixProcesses(t)
	r := newRuntime(t, "acme.crash", nil)
	defer r.Stop()
	startAndWait(t, r)
	_, _ = callSearch(t, r, "x")
	waitState(t, r, StateFailed)
	holdStartingAfterFailure(r)

	_, err := callSearch(t, r, "x")
	message := callError(t, err).Failure.Message
	if !strings.Contains(message, "restarting after a failure") || !strings.Contains(message, "goroutine panic") {
		t.Fatalf("failure = %q; want the restart and the panic that caused it", message)
	}
}

// holdStartingAfterFailure puts a failed runtime in the starting state with no launch that could finish it.
func holdStartingAfterFailure(r *Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = StateStarting
	r.generation++
}
