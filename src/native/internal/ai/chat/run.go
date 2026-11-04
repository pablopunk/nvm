package chat

import "github.com/pablopunk/nvm/native/internal/ai/agent"

// run is one turn in flight. Its loop runs on its own goroutine; pending names the tool call that waits for the user's
// answer. Only the UI thread reads or writes pending.
type run struct {
	turn    *agent.Turn
	pending string
}

// newRun wraps a started turn.
func newRun(turn *agent.Turn) *run {
	return &run{turn: turn}
}

// stop cancels the turn. The loop reports what it had and ends.
func (r *run) stop() {
	r.turn.Cancel()
}
