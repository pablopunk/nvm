package dictation

import "context"

// keepFailed leaves a failed dictation's recording in the history with the reason, and returns the error the user
// reads. Only a recording that is wholly on disk says it was saved.
func (s *Service) keepFailed(op *operation, saved bool, cause error) error {
	if !saved {
		return cause
	}
	s.markFailed(op.id, cause)
	return &SavedError{Err: cause}
}

// keepCancelled leaves a cancelled dictation's recording in the history, marked as cancelled.
func (s *Service) keepCancelled(op *operation, saved bool) {
	if !saved {
		return
	}
	if err := s.cfg.Recorder.MarkFailed(op.id, cancelledRecordingNote); err != nil {
		s.warn("dictation.recording-mark-failed", err)
	}
}

// markFailed writes the reason a saved dictation failed beside its audio. A write that fails is logged.
func (s *Service) markFailed(id string, cause error) {
	if err := s.cfg.Recorder.MarkFailed(id, FailureMessage(cause)); err != nil {
		s.warn("dictation.recording-mark-failed", err)
	}
}

// retryRecording transcribes the loaded audio of saved dictation id under a fresh operation ID, retrying passing
// failures under the policy. The reason of a failure is written beside the audio, which stays until the caller
// deletes it.
func (s *Service) retryRecording(ctx context.Context, id string, files [][]byte, transcriber Transcriber) (string, error) {
	text, err := transcribeFiles(ctx, transcriber, s.cfg.Retry, newRetryOperationID(), false, files, s.retryWarning("dictation.transcribe-retry"))
	if err != nil && ctx.Err() == nil {
		s.markFailed(id, err)
	}
	return text, err
}

// discardRecording drops the saved audio of dictation id once its text is confirmed, or when it holds no sound.
func (s *Service) discardRecording(id string) {
	if s.cfg.Recorder == nil {
		return
	}
	if err := s.cfg.Recorder.Remove(id); err != nil {
		s.warn("dictation.recording-remove-failed", err)
	}
}
