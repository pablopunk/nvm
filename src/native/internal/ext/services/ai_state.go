package services

import (
	"context"
	"strconv"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// aiState is the host side of one extension's AI calls: its open streams and its sessions. Its context ends with the
// extension's services, which stops every stream.
type aiState struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	nextID   uint64
	streams  map[string]*aiStream
	sessions map[string]*aiSession
}

func newAIState() *aiState {
	ctx, cancel := context.WithCancel(context.Background())
	return &aiState{ctx: ctx, cancel: cancel, streams: map[string]*aiStream{}, sessions: map[string]*aiSession{}}
}

// close stops every open stream. The sessions are dropped with the handler.
func (a *aiState) close() {
	a.cancel()
}

// session returns the session named id, starting it with the system text and tier of options when it is new.
func (a *aiState) session(id string, options api.AIOptions) (*aiSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if session, ok := a.sessions[id]; ok {
		return session, nil
	}
	if len(a.sessions) >= maxSessions {
		return nil, aiLimitRefusal("this extension has too many AI sessions; reset one first")
	}
	session := &aiSession{system: options.System, model: options.Model}
	a.sessions[id] = session
	return session, nil
}

func (a *aiState) resetSession(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
}

// addStream stores an opened reply as a stream and starts it. A reply that would exceed the open-stream limit is
// closed and refused.
func (a *aiState) addStream(reply *extai.Reply, session *aiSession, prompt backend.Message) (*aiStream, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.streams) >= maxOpenStreams {
		reply.Close()
		return nil, aiLimitRefusal("this extension has too many open AI streams; finish or abort one first")
	}
	a.nextID++
	stream := newAIStream(strconv.FormatUint(a.nextID, 10), reply, session, prompt)
	a.streams[stream.id] = stream
	return stream, nil
}

func (a *aiState) stream(id string) (*aiStream, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	stream, ok := a.streams[id]
	return stream, ok
}

func (a *aiState) removeStream(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.streams, id)
}

// aiSession is one conversation of an extension. It keeps its last turns, so a follow-up call has the context of the
// ones before it.
type aiSession struct {
	system   string
	model    api.ModelTier
	mu       sync.Mutex
	messages []backend.Message
}

// optionsFor fills the options of a call from the session where the call leaves them empty.
func (s *aiSession) optionsFor(call api.AIOptions) api.AIOptions {
	if call.System == "" {
		call.System = s.system
	}
	if call.Model == "" {
		call.Model = s.model
	}
	return call
}

// history is a copy of the turns the session keeps.
func (s *aiSession) history() []backend.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]backend.Message(nil), s.messages...)
}

// record keeps one finished turn: the user's prompt and the reply it got. Only the newest turns are kept, in pairs.
func (s *aiSession) record(prompt backend.Message, reply string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, prompt, backend.Message{Role: backend.RoleAssistant, Content: backend.TextContent(reply)})
	if len(s.messages) > sessionMessages {
		s.messages = s.messages[len(s.messages)-sessionMessages:]
	}
}
