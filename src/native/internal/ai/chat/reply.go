package chat

import (
	"context"
	"errors"
	"io"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Ending says how a turn ended.
type Ending string

// The endings. Only the stream's end marker finishes a reply; a stream that breaks fails, and a stop cancels.
const (
	EndFinished Ending = "finished"
	EndStopped  Ending = "stopped"
	EndFailed   Ending = "failed"
)

// Outcome is how a turn ended, with the error when it failed.
type Outcome struct {
	Ending Ending
	Err    error
}

// replyStream is the part of a backend stream the reader needs.
type replyStream interface {
	Next() (backend.Event, error)
}

// readReply reads events until the reply ends and passes each text piece to deliver. Reasoning, tool-call and
// usage events carry nothing a conversation shows, so they are dropped.
func readReply(ctx context.Context, stream replyStream, deliver func(text string)) Outcome {
	for {
		event, err := stream.Next()
		switch {
		case err != nil:
			return endingOnError(ctx, err)
		case event.Type == backend.EventDone:
			return Outcome{Ending: EndFinished}
		case event.Type == backend.EventError:
			return endingOnError(ctx, event.Err)
		case event.Type == backend.EventContent && event.Text != "":
			deliver(event.Text)
		}
	}
}

// endingOnError classifies a read that did not reach the end marker: a stop when the turn was cancelled,
// otherwise a failure carrying the error.
func endingOnError(ctx context.Context, err error) Outcome {
	if ctx.Err() != nil {
		return Outcome{Ending: EndStopped}
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = &backend.Error{Kind: backend.KindBadResponse, Message: "stream ended before the end-of-stream marker"}
	}
	return Outcome{Ending: EndFailed, Err: err}
}
