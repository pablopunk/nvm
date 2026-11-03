package backend

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// Stream is an open server-sent event response. Call Next until it returns io.EOF, and always call Close.
// Next is not safe for concurrent use. Close is, and calling it from another goroutine aborts a Next that
// is waiting on the network: that Next then yields an EventError of KindCanceled and then io.EOF.
type Stream struct {
	client       *Client
	ex           exchange
	caller       context.Context
	frames       *frameReader
	pending      []Event
	finished     bool
	finishReason string
	closed       atomic.Bool
	release      sync.Once
}

// openStream sends spec and returns the open stream of a success response. A non-success response is read
// for its envelope and returned as an error before any event is delivered.
func (c *Client) openStream(ctx context.Context, spec requestSpec) (*Stream, error) {
	ex, err := c.send(ctx, spec)
	if err != nil {
		return nil, err
	}
	if ex.status < 200 || ex.status > 299 {
		body, _, readErr := readCapped(ex.body, maxErrorBytes)
		ex.release()
		if readErr != nil {
			return nil, c.readFailure(ctx, ex, readErr)
		}
		return nil, errorFromResponse(ex.status, ex.retryAfter, body)
	}
	return &Stream{client: c, ex: ex, caller: ctx, frames: newFrameReader(ex.body)}, nil
}

// Next returns the next event. It returns io.EOF after the terminal event, and after Close. Failures arrive as
// an EventError with a non-nil Err, not as a Go error, so a caller handles one stream of events.
func (s *Stream) Next() (Event, error) {
	for {
		if len(s.pending) > 0 {
			event := s.pending[0]
			s.pending = s.pending[1:]
			return event, nil
		}
		if s.finished || s.closed.Load() {
			return Event{}, io.EOF
		}
		frame, err := s.frames.next()
		switch {
		case err == nil:
			s.handleFrame(frame)
		case errors.Is(err, io.EOF):
			s.fail(&Error{Kind: KindBadResponse, Status: s.ex.status, Message: "stream ended before the end-of-stream marker"})
		case errors.Is(err, errLineTooLong):
			s.fail(&Error{Kind: KindBadResponse, Status: s.ex.status, Message: "stream line exceeds the size limit", Cause: err})
		default:
			s.fail(s.readError(err))
		}
	}
}

// Close releases the connection. It is safe to call more than once and from any goroutine. Events already
// decoded are dropped.
func (s *Stream) Close() error {
	s.closed.Store(true)
	s.releaseOnce()
	return nil
}

// readError classifies a read failure. A read that fails after Close is a cancellation, not a network fault.
func (s *Stream) readError(err error) *Error {
	if s.closed.Load() {
		return &Error{Kind: KindCanceled, Message: "the stream was closed", Cause: err}
	}
	return s.client.readFailure(s.caller, s.ex, err)
}

// handleFrame decodes one dispatched event and queues what it carries. The end marker and error envelopes
// finish the stream.
func (s *Stream) handleFrame(frame sseFrame) {
	if frame.data == doneMarker {
		s.queue(Event{Type: EventDone, FinishReason: s.finishReason})
		s.finish()
		return
	}
	events, finish, decodeErr := decodeChunk(frame.data)
	if finish != "" {
		s.finishReason = finish
	}
	if decodeErr != nil {
		s.fail(decodeErr)
		return
	}
	for _, event := range events {
		s.queue(event)
	}
}

// fail ends the stream with an error event.
func (s *Stream) fail(err *Error) {
	s.queue(Event{Type: EventError, Err: err})
	s.finish()
}

// queue appends an event to deliver on the next call to Next.
func (s *Stream) queue(event Event) {
	s.pending = append(s.pending, event)
}

// finish marks the stream complete and releases the connection. Events already queued are still delivered.
func (s *Stream) finish() {
	s.finished = true
	s.releaseOnce()
}

// releaseOnce releases the exchange exactly once.
func (s *Stream) releaseOnce() {
	s.release.Do(s.ex.release)
}
