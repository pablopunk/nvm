package child

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteStreamBuffer is how many events wait for a consumer of Events. A consumer that falls further behind misses
// deltas; Result still returns the whole reply.
const remoteStreamBuffer = 4096

// remoteStream is an AIStream whose reply the host streams. A goroutine pulls the events since its last pull and
// delivers them on Events, so Events never waits on the host and Result never waits on a consumer.
type remoteStream struct {
	link   *hostLink
	id     string
	events chan api.AIStreamEvent
	done   chan struct{}
	mu     sync.Mutex
	text   string
	err    error
}

func newRemoteStream(link *hostLink, id string) *remoteStream {
	stream := &remoteStream{link: link, id: id, events: make(chan api.AIStreamEvent, remoteStreamBuffer), done: make(chan struct{})}
	go stream.pull()
	return stream
}

func (s *remoteStream) Events() <-chan api.AIStreamEvent {
	return s.events
}

// Result waits for the end of the reply and returns its text, or the failure that ended it.
func (s *remoteStream) Result() (string, error) {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text, s.err
}

// Abort asks the host to stop the reply. The end comes through Events and Result as an aborted reply.
func (s *remoteStream) Abort() {
	_ = s.link.Call(protocol.ServiceAI, protocol.OpStreamAbort, protocol.AIStreamRef{ID: s.id}, nil)
}

// pull reads batches until the host ends the stream.
func (s *remoteStream) pull() {
	defer close(s.events)
	for {
		var batch protocol.AIStreamBatch
		if err := s.link.Call(protocol.ServiceAI, protocol.OpStreamNext, protocol.AIStreamRef{ID: s.id}, &batch); err != nil {
			s.finish("", err)
			return
		}
		s.deliver(batch.Events)
		if batch.Ended {
			s.finish(batch.Text, batchFailure(batch))
			return
		}
	}
}

// deliver sends events without blocking; see remoteStreamBuffer.
func (s *remoteStream) deliver(events []api.AIStreamEvent) {
	for _, event := range events {
		select {
		case s.events <- event:
		default:
		}
	}
}

// finish records the outcome and releases Result.
func (s *remoteStream) finish(text string, err error) {
	s.mu.Lock()
	s.text, s.err = text, err
	s.mu.Unlock()
	close(s.done)
}

// batchFailure is the error a finished batch carries, or nil when the reply finished.
func batchFailure(batch protocol.AIStreamBatch) error {
	if batch.Failure == nil {
		return nil
	}
	return failureError(batch.Failure)
}
