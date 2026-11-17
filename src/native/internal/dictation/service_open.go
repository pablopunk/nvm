package dictation

import (
	"context"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// openCaptureWithRetry opens the microphone and the encoder, trying again under the retry policy while the failure may
// pass. A Cancel ends the waits between tries.
func (s *Service) openCaptureWithRetry(op *operation, deviceID string) (platform.MicrophoneStream, platform.OpusEncoder, error) {
	var stream platform.MicrophoneStream
	var encoder platform.OpusEncoder
	err := s.cfg.Retry.do(op.ctx, func(int) error {
		var err error
		stream, encoder, err = s.openCapture(op.captureCtx, deviceID)
		return err
	}, s.retryWarning("dictation.start-retry"))
	return stream, encoder, err
}

// openCapture opens the microphone and the encoder for one dictation. Either failure closes what was opened.
func (s *Service) openCapture(ctx context.Context, deviceID string) (platform.MicrophoneStream, platform.OpusEncoder, error) {
	stream, err := s.openMicrophone(ctx, deviceID)
	if err != nil {
		return nil, nil, err
	}
	encoder, err := s.cfg.Codec.NewEncoder(Format())
	if err != nil {
		_ = stream.Close()
		return nil, nil, fmt.Errorf("%w: %w", ErrCodec, err)
	}
	return stream, encoder, nil
}

// openMicrophone opens deviceID. When a chosen device fails for a reason other than a refusal, the default device
// stands in, as the desktop app does; if that fails too, the chosen device's error is returned.
func (s *Service) openMicrophone(ctx context.Context, deviceID string) (platform.MicrophoneStream, error) {
	stream, err := s.cfg.Microphone.Open(ctx, deviceID, Format())
	if err == nil || deviceID == "" || errors.Is(err, platform.ErrDenied) {
		return stream, err
	}
	s.warn("dictation.device-fallback", err)
	if fallback, fallbackErr := s.cfg.Microphone.Open(ctx, "", Format()); fallbackErr == nil {
		return fallback, nil
	}
	return nil, err
}

func (s *Service) warn(event string, err error) {
	if s.cfg.Warn != nil {
		s.cfg.Warn(event, map[string]any{"error": err.Error()})
	}
}
