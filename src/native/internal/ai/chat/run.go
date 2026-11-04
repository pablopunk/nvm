package chat

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// run is one turn in flight. Its loop runs on its own goroutine; pending names the tool call that waits for the user's
// answer. Only the UI thread reads or writes pending and the fields that log the turn.
type run struct {
	turn    *agent.Turn
	pending string
	kind    Kind
	tier    Tier
	started time.Time
	steps   int
	// reasoningSince is when the reasoning of the current model turn began; zero when none is open.
	reasoningSince time.Time
}

// newRun wraps a started turn of a chat of kind on tier, begun at started.
func newRun(turn *agent.Turn, kind Kind, tier Tier, started time.Time) *run {
	return &run{turn: turn, kind: kind, tier: tier, started: started}
}

// stop cancels the turn. The loop reports what it had and ends.
func (r *run) stop() {
	r.turn.Cancel()
}
