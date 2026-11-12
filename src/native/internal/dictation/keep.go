package dictation

import "context"

// keepRecording saves the audio of dictation id before it is sent. A save that fails is logged; the dictation goes on
// without a recording to retry.
func (s *Service) keepRecording(id string, files [][]byte) bool {
	if s.cfg.Recorder == nil {
		return false
	}
	if err := s.cfg.Recorder.Save(id, files); err != nil {
		s.warn("dictation.recording-save-failed", err)
		return false
	}
	return true
}

// markFailed writes the reason a saved dictation failed beside its audio. A write that fails is logged.
func (s *Service) markFailed(id string, cause error) {
	if err := s.cfg.Recorder.MarkFailed(id, FailureMessage(cause)); err != nil {
		s.warn("dictation.recording-mark-failed", err)
	}
}

// retryRecording transcribes the loaded audio of saved dictation id under a fresh operation ID. The reason of a failure
// is written beside the audio, which stays until the caller deletes it.
func (s *Service) retryRecording(ctx context.Context, id string, files [][]byte) (string, error) {
	text, err := transcribeFiles(ctx, s.cfg.Transcriber, s.cfg.Retry, newRetryOperationID(), false, files)
	if err != nil && ctx.Err() == nil {
		s.markFailed(id, err)
	}
	return text, err
}

// discardRecording drops the saved audio of dictation id once its text is confirmed, or when it was cancelled.
func (s *Service) discardRecording(id string, saved bool) {
	if !saved {
		return
	}
	if err := s.cfg.Recorder.Remove(id); err != nil {
		s.warn("dictation.recording-remove-failed", err)
	}
}
