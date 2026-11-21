package dictation

import "errors"

// Cancel ends the current dictation, or the one still opening: the microphone closes, nothing more is sent, the
// backend's preparation is released as a user cancel, and a Stop that is waiting on the backend returns ErrCancelled.
// Audio already captured stays in the recorder, marked as cancelled, so it can still be transcribed from the history.
// Cancel on an idle service does nothing.
func (s *Service) Cancel() {
	s.mu.Lock()
	op, recording := s.cur, s.cur != nil && s.phase == PhaseRecording
	if op == nil {
		op = s.starting
	}
	s.cur, s.starting = nil, nil
	s.phase = PhaseIdle
	s.level.Store(0)
	s.mu.Unlock()
	if op == nil {
		return
	}
	s.releasePreparation(op, true)
	op.endCapture()
	if recording {
		s.cfg.Background("dictation.keep-cancelled", func() { s.closeAbandoned(op, ErrCancelled) })
	}
}

// closeAbandoned finishes a capture nobody will transcribe: it waits for the capture to end, keeps any audio with sound
// under the reason, and drops a recording without any. It returns the error the caller reports.
func (s *Service) closeAbandoned(op *operation, cause error) error {
	op.endCapture()
	<-op.done
	defer func() { _ = op.encoder.Close() }()
	saved := op.journal.finish(op.segments)
	if !op.hasAudioToKeep() {
		s.discardRecording(op.id)
		return cause
	}
	if errors.Is(cause, ErrCancelled) {
		s.keepCancelled(op, saved)
		return cause
	}
	return s.keepFailed(op, saved, cause)
}
