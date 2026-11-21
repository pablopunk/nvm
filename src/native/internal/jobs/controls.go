package jobs

import "time"

// RunNow starts a job now, even while its backoff runs. It fails with ErrDisabled when the job is off or halted.
func (s *Scheduler) RunNow(id string) error {
	return s.start(id, ReasonManual, nil, true)
}

// SetEnabled turns a job on or off and saves the choice. Enabling a halted job clears the halt. Turning a job off
// stops its triggers; a run in progress finishes.
func (s *Scheduler) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	r, ok := s.jobs[id]
	if !ok {
		return ErrUnknownJob
	}
	r.userEnabled = &enabled
	if enabled {
		r.halted = ""
	}
	s.installLocked(r)
	s.saveLocked(r)
	s.notifyLocked()
	return nil
}

// ClearError forgets the last failure, its streak and its backoff. A halted job stays halted until it is enabled.
func (s *Scheduler) ClearError(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.jobs[id]
	if !ok {
		return ErrUnknownJob
	}
	r.lastOutcome, r.lastError = "", ""
	r.consecutive = 0
	r.backoffUntil = time.Time{}
	r.failureTimes = nil
	s.notifyLocked()
	return nil
}
