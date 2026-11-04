package dictation

import (
	"time"
)

// needsReadiness reports whether the device has to deliver sound before the dictation counts as started.
func (s *Service) needsReadiness(deviceID string) bool {
	devices, err := s.cfg.Microphone.Devices()
	if err != nil {
		return false
	}
	return NeedsBluetoothReadiness(DeviceTitle(devices, deviceID))
}

// awaitReady waits until the capture has heard the microphone, the device stops before that, or the timeout passes.
// A failure gives the recording up, as a cancelled one does.
func (s *Service) awaitReady(op *operation) error {
	timeout := s.cfg.ReadyTimeout
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-op.ready:
		return nil
	case <-op.done:
		select {
		case <-op.ready:
			return nil
		default:
		}
	case <-timer.C:
	}
	return s.abandonStart(op)
}

// abandonStart ends a start that never became ready: the stream closes, the preparation is released, and the service
// goes idle. When a Cancel got there first, the start reports the cancellation instead.
func (s *Service) abandonStart(op *operation) error {
	s.mu.Lock()
	if s.cur != op {
		s.mu.Unlock()
		return ErrCancelled
	}
	s.cur = nil
	s.phase = PhaseIdle
	s.mu.Unlock()
	s.releasePreparation(op, false)
	_ = op.stream.Close()
	<-op.done
	_ = op.encoder.Close()
	return ErrNotReady
}
