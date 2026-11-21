package dictation

import "math"

// publishOperationLevel ignores late samples from an operation that has been cancelled or replaced.
func (s *Service) publishOperationLevel(op *operation, level float64) {
	s.mu.Lock()
	if s.cur != op || s.phase != PhaseRecording {
		s.mu.Unlock()
		return
	}
	s.level.Store(math.Float64bits(level))
	s.mu.Unlock()
	if op.levelSink != nil {
		op.levelSink(level, op.id, func() bool { return s.operationRecording(op) })
	}
	if s.cfg.OnLevel != nil {
		s.cfg.OnLevel(level, func() bool { return s.operationRecording(op) })
	}
}

func (s *Service) operationRecording(op *operation) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur == op && s.phase == PhaseRecording
}
