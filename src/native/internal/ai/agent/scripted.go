package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// ScriptedTurn is one scripted model reply: the chunks it yields in order, then either io.EOF (a complete reply) or the
// error that breaks it.
type ScriptedTurn struct {
	Chunks []ModelChunk
	Err    error
}

// ScriptedModel replays scripted replies, one per model turn, and records what it was asked. A turn past the script
// fails the test with ErrScriptExhausted, so a loop that runs too long shows up at once.
type ScriptedModel struct {
	mu       sync.Mutex
	turns    []ScriptedTurn
	requests []ScriptedRequest
	closes   int
}

// ScriptedRequest is what the loop sent for one model turn.
type ScriptedRequest struct {
	Messages []backend.Message
	Tools    json.RawMessage
}

// ErrScriptExhausted is the failure of a model turn the script did not cover.
var ErrScriptExhausted = errors.New("agent: the scripted model has no turn left")

// NewScriptedModel returns a model that replays turns in order.
func NewScriptedModel(turns ...ScriptedTurn) *ScriptedModel {
	return &ScriptedModel{turns: turns}
}

// Stream answers one model turn from the script.
func (m *ScriptedModel) Stream(ctx context.Context, messages []backend.Message, tools json.RawMessage) (ModelStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, ScriptedRequest{Messages: append([]backend.Message(nil), messages...), Tools: tools})
	if len(m.requests) > len(m.turns) {
		return nil, ErrScriptExhausted
	}
	return &scriptedStream{ctx: ctx, model: m, turn: m.turns[len(m.requests)-1]}, nil
}

// Requests returns what the loop sent, one entry per model turn.
func (m *ScriptedModel) Requests() []ScriptedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ScriptedRequest(nil), m.requests...)
}

// Closes returns how many model streams were closed. The loop closes every stream it opens, so it equals the number
// of turns that were streamed.
func (m *ScriptedModel) Closes() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closes
}

type scriptedStream struct {
	ctx   context.Context
	model *ScriptedModel
	turn  ScriptedTurn
	next  int
}

// Next yields the next scripted chunk, or the turn's end, and stops when the stream's context is cancelled.
func (s *scriptedStream) Next() (ModelChunk, error) {
	if s.ctx.Err() != nil {
		return ModelChunk{}, s.ctx.Err()
	}
	if s.next < len(s.turn.Chunks) {
		chunk := s.turn.Chunks[s.next]
		s.next++
		return chunk, nil
	}
	if s.turn.Err != nil {
		return ModelChunk{}, s.turn.Err
	}
	return ModelChunk{}, io.EOF
}

// Close counts the close.
func (s *scriptedStream) Close() error {
	s.model.mu.Lock()
	defer s.model.mu.Unlock()
	s.model.closes++
	return nil
}
