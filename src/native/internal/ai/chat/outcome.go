package chat

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

// ErrEmptyReply is the failure of a reply that finished without any text.
var ErrEmptyReply = errors.New("chat: the reply had no text")

// Ending says how a turn ended.
type Ending string

// The endings. Only a turn that reached its final answer finishes; a failure keeps what arrived, and a stop cancels.
const (
	EndFinished Ending = "finished"
	EndStopped  Ending = "stopped"
	EndFailed   Ending = "failed"
)

// Outcome is how a turn ended, with the error when it failed and the tokens it used.
type Outcome struct {
	Ending Ending
	Err    error
	Usage  agent.Usage
}

// failed is the outcome of a turn that ended with err.
func failed(err error) Outcome {
	return Outcome{Ending: EndFailed, Err: err}
}

// outcomeOf returns the outcome a terminal event of a turn stands for. Other events leave the outcome as it was.
func outcomeOf(event agent.Event, current Outcome) Outcome {
	switch event.Kind {
	case agent.EventDone:
		return Outcome{Ending: EndFinished, Usage: event.Usage}
	case agent.EventAborted:
		return Outcome{Ending: EndStopped, Usage: event.Usage}
	case agent.EventError:
		return Outcome{Ending: EndFailed, Err: event.Err, Usage: event.Usage}
	}
	return current
}
