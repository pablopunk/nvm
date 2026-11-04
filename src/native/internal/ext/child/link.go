package child

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// hostLink is the child's end of the service exchange. Calls write a ServiceCall on stdout and wait for the
// ServiceReply with the same id, which the reader delivers from stdin. Every write to stdout goes through the
// link, so responses and service calls never interleave.
type hostLink struct {
	mu      sync.Mutex
	out     io.Writer
	nextID  uint64
	pending map[uint64]chan protocol.ServiceReply
	closed  bool
}

func newHostLink(out io.Writer) *hostLink {
	return &hostLink{out: out, pending: map[uint64]chan protocol.ServiceReply{}}
}

// send writes one JSON value as one protocol line.
func (l *hostLink) send(v any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	writeLineLocked(l.out, v)
}

// Call runs one host operation and decodes its result into into, which may be nil. A host failure comes back as
// an error that wraps the matching api sentinel, so extension code can classify it with errors.Is.
func (l *hostLink) Call(service, op string, args any, into any) error {
	raw, err := marshalArgs(args)
	if err != nil {
		return fmt.Errorf("%s.%s: arguments cannot be encoded: %w", service, op, api.ErrInvalidInput)
	}
	id, reply, err := l.register()
	if err != nil {
		return err
	}
	defer l.forget(id)
	l.send(protocol.ServiceCall{CallID: id, Service: service, Op: op, Args: raw})
	answer, open := <-reply
	if !open {
		return fmt.Errorf("%s.%s: the host closed the extension connection: %w", service, op, api.ErrNotAvailable)
	}
	if answer.Failure != nil {
		return failureError(answer.Failure)
	}
	if into != nil && len(answer.Data) > 0 {
		if err := json.Unmarshal(answer.Data, into); err != nil {
			return fmt.Errorf("%s.%s: the host answered with data that does not fit: %w", service, op, api.ErrInvalidInput)
		}
	}
	return nil
}

func marshalArgs(args any) (json.RawMessage, error) {
	if args == nil {
		return nil, nil
	}
	return json.Marshal(args)
}

// register reserves a call id and the channel its reply arrives on.
func (l *hostLink) register() (uint64, <-chan protocol.ServiceReply, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return 0, nil, api.ErrNotAvailable
	}
	l.nextID++
	reply := make(chan protocol.ServiceReply, 1)
	l.pending[l.nextID] = reply
	return l.nextID, reply, nil
}

func (l *hostLink) forget(id uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.pending, id)
}

// deliver hands a reply to the call waiting for it. A reply nobody waits for is dropped.
func (l *hostLink) deliver(reply protocol.ServiceReply) {
	l.mu.Lock()
	defer l.mu.Unlock()
	waiting, ok := l.pending[reply.ReplyTo]
	if !ok {
		return
	}
	delete(l.pending, reply.ReplyTo)
	waiting <- reply
}

// close ends the exchange when the host's stdin ends: waiting calls see a closed channel and fail.
func (l *hostLink) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	for id, waiting := range l.pending {
		close(waiting)
		delete(l.pending, id)
	}
}

// failureError turns a host failure into an error that wraps the api sentinel of its code.
func failureError(failure *api.Failure) error {
	sentinel := sentinelFor(failure.Code)
	if sentinel == nil {
		return errors.New(failure.Message)
	}
	return fmt.Errorf("%s: %w", failure.Message, sentinel)
}

func sentinelFor(code api.ErrorCode) error {
	switch code {
	case api.CodeUnavailable:
		return api.ErrNotAvailable
	case api.CodePermission:
		return api.ErrPermission
	case api.CodeInvalid:
		return api.ErrInvalidInput
	case api.CodeNotFound:
		return api.ErrNotFound
	case api.CodeQuota:
		return api.ErrQuotaExceeded
	}
	return nil
}
