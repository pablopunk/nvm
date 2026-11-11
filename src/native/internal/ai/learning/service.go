package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

const maxReviewBytes = 16 << 10

// Service serializes trace invalidation, user mutations and publication of model proposals.
// Wake schedules Run on the existing host job runner; it must not block on model work.
type Service struct {
	mu      sync.Mutex
	rules   *Rules
	traces  *Traces
	ask     func(context.Context, string) (string, error)
	newID   func() (string, error)
	wake    func()
	pending map[string]bool
	waiting map[string]bool
	epoch   uint64
}

func NewService(rules *Rules, ask func(context.Context, string) (string, error), newID func() (string, error), wake func()) *Service {
	return &Service{rules: rules, traces: NewTraces(), ask: ask, newID: newID, wake: wake, pending: map[string]bool{}, waiting: map[string]bool{}}
}
func (s *Service) Message(id, role, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces.Message(id, role, text)
}
func (s *Service) Outcome(id, outcome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces.Outcome(id, outcome)
}

// Exit waits for an active turn to settle before queuing its review.
func (s *Service) Exit(id string, busy bool) {
	s.mu.Lock()
	if busy {
		s.waiting[id] = true
		s.mu.Unlock()
		return
	}
	_, eligible := s.traces.Snapshot(id)
	if eligible {
		if len(s.pending) >= maxTraces && !s.pending[id] {
			for old := range s.pending {
				delete(s.pending, old)
				break
			}
		}
		s.pending[id] = true
	}
	wake := s.wake
	s.mu.Unlock()
	if eligible && wake != nil {
		wake()
	}
}
func (s *Service) Settled(id string) {
	s.mu.Lock()
	waiting := s.waiting[id]
	delete(s.waiting, id)
	s.mu.Unlock()
	if waiting {
		s.Exit(id, false)
	}
}
func (s *Service) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
	delete(s.waiting, id)
	s.traces.Remove(id)
	s.epoch++
}
func (s *Service) Snapshot() (Document, error) { return s.rules.Snapshot() }
func (s *Service) Edit(expected uint64, rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rules.Edit(expected, rule)
}
func (s *Service) Delete(expected uint64, id string, revision uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rules.Delete(expected, id, revision); err != nil {
		return err
	}
	s.epoch++
	s.traces.Clear()
	s.pending = map[string]bool{}
	s.waiting = map[string]bool{}
	return nil
}

// Run processes a bounded pending snapshot. Stale traces or user changes discard model output.
func (s *Service) Run(ctx context.Context) error {
	defer func() {
		s.mu.Lock()
		pending, wake := len(s.pending) > 0, s.wake
		s.mu.Unlock()
		if pending && wake != nil && ctx.Err() == nil {
			wake()
		}
	}()
	return s.runOne(ctx)
}

func (s *Service) runOne(ctx context.Context) error {
	if s.ask == nil || s.newID == nil {
		return errors.New("learning: review unavailable")
	}
	s.mu.Lock()
	id := ""
	for candidate := range s.pending {
		id = candidate
		break
	}
	if id == "" {
		s.mu.Unlock()
		return nil
	}
	delete(s.pending, id)
	trace, ok := s.traces.Snapshot(id)
	snapshot, err := s.rules.Snapshot()
	epoch := s.epoch
	s.mu.Unlock()
	if !ok || err != nil {
		return err
	}
	prompt, err := reviewInput(trace, snapshot)
	if err != nil {
		return err
	}
	output, err := s.ask(ctx, prompt)
	if err != nil {
		return errors.New("learning: review failed")
	}
	proposals, err := decodeProposals(output)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, eligible := s.traces.Snapshot(id)
	if s.epoch != epoch || !eligible || current.Sequence != trace.Sequence {
		return ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.rules.ApplyReview(snapshot.Revision, proposals, s.newID); err != nil {
		return err
	}
	s.traces.markReviewed(id, trace.Sequence)
	return nil
}
func reviewInput(trace Trace, doc Document) (string, error) {
	if len(doc.Rules) > 20 {
		doc.Rules = doc.Rules[:20]
	}
	data, err := json.Marshal(struct {
		Trace Trace  `json:"trace"`
		Rules []Rule `json:"rules"`
	}{trace, doc.Rules})
	if err != nil {
		return "", err
	}
	for len(data) > maxReviewBytes && len(trace.Messages) > 0 {
		trace.Messages = trace.Messages[1:]
		data, err = json.Marshal(struct {
			Trace Trace  `json:"trace"`
			Rules []Rule `json:"rules"`
		}{trace, doc.Rules})
		if err != nil {
			return "", err
		}
	}
	if len(data) > maxReviewBytes {
		return "", errors.New("learning: review input exceeds limit")
	}
	return string(data), nil
}
func decodeProposals(output string) ([]Proposal, error) {
	if len(output) > maxReviewBytes {
		return nil, ErrInvalid
	}
	var envelope struct {
		Proposals []Proposal `json:"proposals"`
	}
	dec := json.NewDecoder(bytes.NewBufferString(output))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return nil, ErrInvalid
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, ErrInvalid
	}
	if envelope.Proposals == nil || len(envelope.Proposals) > 20 {
		return nil, ErrInvalid
	}
	return envelope.Proposals, nil
}
