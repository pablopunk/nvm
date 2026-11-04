package dictation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Phase is where the one dictation the service runs stands.
type Phase string

// The dictation phases. Recording covers the capture; transcribing covers the backend calls after Stop.
const (
	PhaseIdle         Phase = "idle"
	PhaseRecording    Phase = "recording"
	PhaseTranscribing Phase = "transcribing"
)

// Errors the service returns. Platform and backend errors pass through unchanged, so FailureMessage can explain them.
var (
	ErrBusy         = errors.New("dictation is already running")
	ErrNotRecording = errors.New("dictation is not recording")
	ErrCancelled    = errors.New("dictation was cancelled")
	ErrCodec        = errors.New("dictation needs the Opus codec")
)

// Config holds the ports a Service runs on. OnLevel receives the meter level about every 100 ms while recording,
// from the capture goroutine, so it must only hand the value to the UI.
type Config struct {
	Microphone  platform.Microphone
	Codec       platform.OpusCodec
	Transcriber Transcriber
	OnLevel     func(level float64)
}

// Snapshot is what the indicator draws: the phase and the level of the last captured frames.
type Snapshot struct {
	Phase Phase
	Level float64
}

// Service runs one dictation at a time: Start opens the microphone and begins capture, Stop ends it and returns
// the transcript, Cancel drops it. It is safe for concurrent use; Stop blocks on the backend, so callers run it off
// the UI thread.
type Service struct {
	cfg   Config
	mu    sync.Mutex
	phase Phase
	cur   *operation
	level atomic.Uint64
}

// NewService returns an idle service.
func NewService(cfg Config) *Service {
	return &Service{cfg: cfg, phase: PhaseIdle}
}

// Snapshot returns the phase and the latest level.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	phase := s.phase
	s.mu.Unlock()
	return Snapshot{Phase: phase, Level: math.Float64frombits(s.level.Load())}
}

// Start opens deviceID, or the default device when it is "", and starts capturing. It returns once the stream is
// open, so a missing or refused microphone is reported here and not later.
func (s *Service) Start(deviceID string) error {
	s.mu.Lock()
	if s.phase != PhaseIdle {
		s.mu.Unlock()
		return ErrBusy
	}
	s.phase = PhaseRecording
	s.mu.Unlock()

	op, err := s.open(deviceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.phase = PhaseIdle
		return err
	}
	s.cur = op
	return nil
}

// open starts the stream, the encoder and the capture goroutine for one dictation.
func (s *Service) open(deviceID string) (*operation, error) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := s.cfg.Microphone.Open(ctx, deviceID, Format())
	if err != nil {
		cancel()
		return nil, err
	}
	encoder, err := s.cfg.Codec.NewEncoder(Format())
	if err != nil {
		_ = stream.Close()
		cancel()
		return nil, fmt.Errorf("%w: %w", ErrCodec, err)
	}
	op := &operation{id: newOperationID(), ctx: ctx, cancel: cancel, stream: stream, encoder: encoder, done: make(chan struct{}), onLevel: s.publishLevel}
	safego.Go("dictation.capture", func(context.Context) { op.capture() })
	s.prepare(op)
	return op, nil
}

// prepare warms the backend's admission while the user speaks. It runs on its own goroutine, and the transcription
// waits for it to finish, so the backend sees the preparation first. Its failure is ignored, as the transcription
// request repeats the check.
func (s *Service) prepare(op *operation) {
	op.prepared = make(chan struct{})
	if s.cfg.Transcriber == nil {
		close(op.prepared)
		return
	}
	safego.Go("dictation.prepare", func(context.Context) {
		defer close(op.prepared)
		_ = s.cfg.Transcriber.Prepare(op.ctx, op.id)
	})
}

func (s *Service) publishLevel(level float64) {
	s.level.Store(math.Float64bits(level))
	if s.cfg.OnLevel != nil {
		s.cfg.OnLevel(level)
	}
}

// Stop ends the recording, sends the segments to the backend and returns the joined text. An empty text means no
// speech was captured or the speech was silent; the caller shows that as a state, not as an error.
func (s *Service) Stop() (string, error) {
	s.mu.Lock()
	op := s.cur
	if s.phase != PhaseRecording || op == nil {
		s.mu.Unlock()
		return "", ErrNotRecording
	}
	s.phase = PhaseTranscribing
	s.mu.Unlock()

	_ = op.stream.Close()
	<-op.done
	s.publishLevel(0)
	text, err := s.finish(op)
	op.cancel()
	s.release(op)
	return text, err
}

// finish turns a captured operation into text. A microphone that failed mid-recording fails the dictation.
func (s *Service) finish(op *operation) (string, error) {
	defer func() { _ = op.encoder.Close() }()
	if op.failure != nil {
		return "", op.failure
	}
	if !op.hasSpeechAudio() {
		return "", nil
	}
	if s.cfg.Transcriber == nil {
		return "", errors.New("dictation has no transcription service")
	}
	select {
	case <-op.prepared:
	case <-op.ctx.Done():
		return "", ErrCancelled
	}
	text, err := transcribeSegments(op.ctx, s.cfg.Transcriber, op)
	if op.ctx.Err() != nil {
		return "", ErrCancelled
	}
	return text, err
}

// release returns the service to idle when op is still the current operation. A cancel may already have done so.
func (s *Service) release(op *operation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == op {
		s.cur = nil
		s.phase = PhaseIdle
	}
}

// Cancel drops the current dictation: the microphone closes, nothing is transcribed, and a Stop that is waiting on
// the backend returns ErrCancelled. Cancel on an idle service does nothing.
func (s *Service) Cancel() {
	s.mu.Lock()
	op := s.cur
	s.cur = nil
	s.phase = PhaseIdle
	s.mu.Unlock()
	if op == nil {
		return
	}
	op.cancel()
	_ = op.stream.Close()
	s.publishLevel(0)
}

// newOperationID returns a random key for one dictation, short enough for the backend's idempotency header.
func newOperationID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return "dictation-" + hex.EncodeToString(raw[:])
}
