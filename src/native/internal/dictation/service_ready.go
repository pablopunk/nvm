package dictation

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// needsReadiness reports whether the device has to deliver sound before the dictation counts as started. The device
// list is read under the retry policy; a list that still cannot be read is logged and the start goes on without the
// wait.
func (s *Service) needsReadiness(ctx context.Context, deviceID string) bool {
	var devices []platform.MicrophoneDevice
	err := s.cfg.Retry.do(ctx, func(int) error {
		var err error
		devices, err = s.cfg.Microphone.Devices()
		return err
	}, s.retryWarning("dictation.devices-retry"))
	if err != nil {
		s.warn("dictation.devices-failed", err)
		return false
	}
	return NeedsBluetoothReadiness(DeviceTitle(devices, deviceID))
}

// awaitReady waits until the capture has heard the microphone. A wait that times out opens the microphone again, under
// the retry policy; once the tries are spent, or the capture ends first, the start fails with ErrNotReady and any sound
// it caught stays in the history.
func (s *Service) awaitReady(op *operation) error {
	timeout := s.cfg.ReadyTimeout
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	onRetry := s.retryWarning("dictation.ready-retry")
	for attempt := 1; ; attempt++ {
		switch s.waitReady(op, timeout) {
		case readyHeard:
			return nil
		case readyCaptureEnded:
			return s.abandonStart(op)
		}
		if attempt >= op.retry.attempts() {
			return s.abandonStart(op)
		}
		onRetry(attempt, ErrNotReady)
		if op.retry.sleep(op.captureCtx, op.retry.wait(attempt)) != nil {
			return s.abandonStart(op)
		}
		op.restartStream()
	}
}

type readyOutcome int

const (
	readyHeard readyOutcome = iota
	readyCaptureEnded
	readyTimedOut
)

// waitReady waits one readiness timeout for the capture to hear the device.
func (s *Service) waitReady(op *operation, timeout time.Duration) readyOutcome {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-op.ready:
		return readyHeard
	case <-op.done:
		select {
		case <-op.ready:
			return readyHeard
		default:
			return readyCaptureEnded
		}
	case <-timer.C:
		return readyTimedOut
	}
}

// abandonStart ends a start that never became ready: the capture stops, the preparation is released, the service goes
// idle, and audio with sound is kept with the reason. When a Cancel got there first, the start reports the
// cancellation instead, and the cancel keeps the audio.
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
	return s.closeAbandoned(op, ErrNotReady)
}
