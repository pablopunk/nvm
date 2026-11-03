package spike

import (
	"context"
	"io"
	"sync"
)

// MockTurn is one scripted assistant turn: its chunks in order, then either a stall until the request context ends
// (Stall) or the error Err, or a normal end when neither is set.
type MockTurn struct {
	Chunks []ModelChunk
	Stall  bool
	Err    error
}

// MockModel replays scripted turns in order and records every request. It makes no network call and reads no clock.
// Once the script is spent it fails with CodeMockExhausted, unless it was built to repeat its last turn forever.
type MockModel struct {
	mu       sync.Mutex
	turns    []MockTurn
	repeat   *MockTurn
	next     int
	requests []ModelRequest
	closes   int
}

// NewMockModel returns a model that replays turns in order.
func NewMockModel(turns ...MockTurn) *MockModel {
	return &MockModel{turns: turns}
}

// NewRepeatingMockModel returns a model that answers every request with turn, so a loop never runs out of script.
func NewRepeatingMockModel(turn MockTurn) *MockModel {
	return &MockModel{repeat: &turn}
}

// Stream opens the next scripted turn.
func (m *MockModel) Stream(ctx context.Context, request ModelRequest) (ModelStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, request)
	turn, ok := m.nextTurn()
	if !ok {
		return nil, &ModelError{Code: CodeMockExhausted, Message: "the mock model has no scripted turn left"}
	}
	return &mockStream{ctx: ctx, turn: turn, owner: m}, nil
}

// Requests returns a copy of every request the model received, oldest first.
func (m *MockModel) Requests() []ModelRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ModelRequest(nil), m.requests...)
}

// Closes reports how many streams were closed.
func (m *MockModel) Closes() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closes
}

func (m *MockModel) nextTurn() (MockTurn, bool) {
	if m.next < len(m.turns) {
		turn := m.turns[m.next]
		m.next++
		return turn, true
	}
	if m.repeat != nil {
		return *m.repeat, true
	}
	return MockTurn{}, false
}

func (m *MockModel) noteClose() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closes++
}

type mockStream struct {
	ctx   context.Context
	turn  MockTurn
	owner *MockModel
	pos   int
}

func (s *mockStream) Next() (ModelChunk, error) {
	if s.pos < len(s.turn.Chunks) {
		chunk := s.turn.Chunks[s.pos]
		s.pos++
		return chunk, nil
	}
	if s.turn.Stall {
		<-s.ctx.Done()
		return ModelChunk{}, s.ctx.Err()
	}
	if s.turn.Err != nil {
		return ModelChunk{}, s.turn.Err
	}
	return ModelChunk{}, io.EOF
}

func (s *mockStream) Close() error {
	s.owner.noteClose()
	return nil
}
