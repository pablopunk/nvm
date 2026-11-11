package learning

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
)

const maxReviewBytes = 16 << 10

// Service serializes trace invalidation, user mutations and publication of model proposals.
// Wake schedules Run on the existing host job runner; it must not block on model work.
type Service struct {
	mu           sync.Mutex
	committed    atomic.Pointer[Document]
	rules        *Rules
	traces       *Traces
	ask          func(context.Context, string) (string, error)
	newID        func() (string, error)
	wake         func()
	pending      map[string]bool
	waiting      map[string]bool
	inflight     map[string]bool
	tracked      map[string]bool
	trackedOrder []string
	overflow     atomic.Bool
	epoch        atomic.Uint64
}

func NewService(rules *Rules, ask func(context.Context, string) (string, error), newID func() (string, error), wake func()) *Service {
	s := &Service{rules: rules, traces: NewTraces(), ask: ask, newID: newID, wake: wake, pending: map[string]bool{}, waiting: map[string]bool{}, inflight: map[string]bool{}, tracked: map[string]bool{}}
	if doc, err := rules.Snapshot(); err == nil {
		s.committed.Store(&doc)
	}
	return s
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

// Epoch is the current deletion/removal generation, read without waiting for durable I/O.
func (s *Service) Epoch() uint64 { return s.epoch.Load() }

// CaptureEpoch excludes a turn captured before a deletion even if the mailbox was delayed.
func (s *Service) CaptureEpoch(id string, epoch uint64, messages []Message, outcomes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch.Load() != epoch {
		return
	}
	for _, message := range messages {
		s.traces.Message(id, message.Role, message.Text)
	}
	for _, outcome := range outcomes {
		s.traces.Outcome(id, outcome)
	}
}

// Exit waits for an active turn to settle before queuing its review.
func (s *Service) Exit(id string, busy bool) {
	if id == "" {
		return
	}
	s.mu.Lock()
	if !s.tracked[id] {
		if len(s.tracked) >= maxTraces {
			evicted := ""
			for _, old := range s.trackedOrder {
				if !s.inflight[old] {
					evicted = old
					break
				}
			}
			if evicted == "" {
				if s.overflow.CompareAndSwap(false, true) {
					log.Print("builder learnings: review capacity reached")
				}
				s.mu.Unlock()
				return
			}
			delete(s.pending, evicted)
			delete(s.waiting, evicted)
			s.traces.Remove(evicted)
			s.forgetTracked(evicted)
		}
		s.tracked[id] = true
		s.trackedOrder = append(s.trackedOrder, id)
	}
	if busy {
		s.waiting[id] = true
		s.mu.Unlock()
		return
	}
	_, eligible := s.traces.Snapshot(id)
	if !eligible && !s.inflight[id] {
		s.forgetTracked(id)
	}
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
	if !s.inflight[id] {
		s.forgetTracked(id)
	}
	s.traces.Remove(id)
	s.epoch.Add(1)
}
func (s *Service) Snapshot() (Document, error) {
	doc := s.committed.Load()
	if doc == nil {
		return Document{}, errors.New("learning: committed snapshot unavailable")
	}
	copy := *doc
	copy.Rules = append([]Rule(nil), doc.Rules...)
	copy.Tombstones = append([]string(nil), doc.Tombstones...)
	return copy, nil
}
func (s *Service) publish() error {
	doc, err := s.rules.Snapshot()
	if err != nil {
		return err
	}
	s.committed.Store(&doc)
	return nil
}
func (s *Service) Edit(expected uint64, rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rules.Edit(expected, rule); err != nil {
		return err
	}
	return s.publish()
}
func (s *Service) Delete(expected uint64, id string, revision uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rules.Delete(expected, id, revision); err != nil {
		return err
	}
	if err := s.publish(); err != nil {
		return err
	}
	s.epoch.Add(1)
	s.traces.Clear()
	s.pending = map[string]bool{}
	s.waiting = map[string]bool{}
	for id := range s.tracked {
		if !s.inflight[id] {
			s.forgetTracked(id)
		}
	}
	return nil
}

// forgetTracked runs under mu and keeps the bounded eviction order in step with tracking.
func (s *Service) forgetTracked(id string) {
	delete(s.tracked, id)
	for i, entry := range s.trackedOrder {
		if entry == id {
			s.trackedOrder = append(s.trackedOrder[:i], s.trackedOrder[i+1:]...)
			break
		}
	}
}

// Run processes a bounded pending snapshot. Stale traces or user changes discard model output.
func (s *Service) Run(ctx context.Context) error { return s.runOne(ctx) }

func (s *Service) finishReview(id string) {
	s.mu.Lock()
	delete(s.inflight, id)
	if !s.pending[id] && !s.waiting[id] {
		s.forgetTracked(id)
	}
	pending := len(s.pending) > 0
	wake := s.wake
	s.mu.Unlock()
	if pending && wake != nil {
		wake()
	}
}

func (s *Service) runOne(ctx context.Context) error {
	if s.ask == nil || s.newID == nil {
		return errors.New("learning: review unavailable")
	}
	s.mu.Lock()
	id := ""
	for candidate := range s.pending {
		if !s.inflight[candidate] {
			id = candidate
			break
		}
	}
	if id == "" {
		s.mu.Unlock()
		return nil
	}
	delete(s.pending, id)
	s.inflight[id] = true
	trace, ok := s.traces.Snapshot(id)
	snapshot, err := s.Snapshot()
	epoch := s.epoch.Load()
	s.mu.Unlock()
	defer s.finishReview(id)
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
	if s.epoch.Load() != epoch || !eligible || current.Sequence != trace.Sequence {
		return ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.rules.ApplyReview(snapshot.Revision, proposals, s.newID); err != nil {
		return err
	}
	if err := s.publish(); err != nil {
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
