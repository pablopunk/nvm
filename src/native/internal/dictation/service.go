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
	"time"

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
	ErrNoRecordings = errors.New("dictation has no recording store")
	ErrNotReady     = errors.New("dictation microphone did not become ready")
)

// defaultReadyTimeout bounds the wait for a Bluetooth microphone to start picking up sound.
const defaultReadyTimeout = 7 * time.Second

// Config holds the ports a Service runs on. OnLevel receives the meter level about every 100 ms while recording,
// from the capture goroutine, so it must only hand the value to the UI. Background runs the backend calls that
// nobody waits for (preparation and its cancellation); nil runs them on a supervised goroutine. Warn logs a
// recovery the user does not see, such as the default microphone standing in for a chosen one that failed.
type Config struct {
	Microphone  platform.Microphone
	Codec       platform.OpusCodec
	Transcriber Transcriber
	OnLevel     func(level float64)
	Background  func(name string, fn func())
	Warn        func(event string, fields map[string]any)
	// Recorder keeps the audio until the text is in. Nil keeps no audio, so nothing can be retried.
	Recorder Recorder
	// ReadyTimeout bounds the readiness wait; zero means defaultReadyTimeout.
	ReadyTimeout time.Duration
}

// Snapshot is what the indicator draws: the phase and the level of the last captured frames.
type Snapshot struct {
	Phase Phase
	Level float64
}

// Service runs one dictation at a time: Start prepares the backend and opens the microphone, Stop ends the capture and
// returns the transcript, Cancel drops it. It is safe for concurrent use; Stop blocks on the backend, so callers run
// it off the UI thread.
type Service struct {
	cfg      Config
	mu       sync.Mutex
	phase    Phase
	cur      *operation
	starting *operation
	level    atomic.Uint64
}

// NewService returns an idle service.
func NewService(cfg Config) *Service {
	if cfg.Background == nil {
		cfg.Background = supervisedBackground
	}
	return &Service{cfg: cfg, phase: PhaseIdle}
}

func supervisedBackground(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() })
}

// Snapshot returns the phase and the latest level.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	phase := s.phase
	s.mu.Unlock()
	return Snapshot{Phase: phase, Level: math.Float64frombits(s.level.Load())}
}

// Start prepares the backend, opens deviceID (or the default device when it is "") and starts capturing. It returns
// once the stream is open, so a missing or refused microphone is reported here and not later. A Bluetooth microphone
// also has to start delivering sound first (ErrNotReady when it does not), so its first words are not lost. A Cancel
// while the microphone is still opening ends the start with ErrCancelled, and the stream that opened late is closed.
func (s *Service) Start(deviceID string) error {
	s.mu.Lock()
	if s.phase != PhaseIdle {
		s.mu.Unlock()
		return ErrBusy
	}
	s.phase = PhaseRecording
	op := newOperation()
	op.onLevel = s.publishLevel
	s.starting = op
	s.mu.Unlock()

	if s.needsReadiness(deviceID) {
		op.ready = make(chan struct{})
		op.tracker = &readinessTracker{}
	}
	s.prepare(op)
	stream, encoder, err := s.openCapture(op.ctx, deviceID)

	s.mu.Lock()
	if s.starting != op {
		s.mu.Unlock()
		if err == nil {
			_ = stream.Close()
			_ = encoder.Close()
		}
		return ErrCancelled
	}
	s.starting = nil
	if err != nil {
		s.releasePreparation(op, false)
		s.phase = PhaseIdle
		s.mu.Unlock()
		return err
	}
	op.stream, op.encoder = stream, encoder
	s.cur = op
	safego.Go("dictation.capture", func(context.Context) { op.capture() })
	s.mu.Unlock()
	if op.ready == nil {
		return nil
	}
	return s.awaitReady(op)
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

// prepare warms the backend's admission while the user speaks. The transcription waits for it to finish, so the
// backend sees the preparation first. A failed preparation is recorded as not prepared and otherwise ignored; the
// transcription request repeats the admission check.
func (s *Service) prepare(op *operation) {
	op.prepared = make(chan struct{})
	if s.cfg.Transcriber == nil {
		close(op.prepared)
		return
	}
	s.cfg.Background("dictation.prepare", func() {
		defer close(op.prepared)
		op.preparedOK = s.cfg.Transcriber.Prepare(op.ctx, op.id) == nil
	})
}

// releasePreparation cancels op's context and releases its backend admission once the preparation has finished.
func (s *Service) releasePreparation(op *operation, userCancelled bool) {
	op.cancel()
	if s.cfg.Transcriber == nil {
		return
	}
	s.cfg.Background("dictation.cancel-preparation", func() {
		<-op.prepared
		_ = s.cfg.Transcriber.CancelPreparation(context.Background(), op.id, userCancelled)
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
	if err != nil && !errors.Is(err, ErrCancelled) || !op.hasSpeechAudio() {
		s.releasePreparation(op, false)
	}
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
	files := segmentFiles(op)
	saved := s.keepRecording(op.id, files)
	text, err := transcribeFiles(op.ctx, s.cfg.Transcriber, op.id, op.preparedOK, files)
	if op.ctx.Err() != nil {
		s.discardRecording(op.id, saved)
		return "", ErrCancelled
	}
	if err != nil {
		if saved {
			return "", &SavedError{Err: err}
		}
		return "", err
	}
	s.discardRecording(op.id, saved)
	return text, nil
}

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

// discardRecording drops the saved audio of dictation id once its text is confirmed, or when it was cancelled.
func (s *Service) discardRecording(id string, saved bool) {
	if !saved {
		return
	}
	if err := s.cfg.Recorder.Remove(id); err != nil {
		s.warn("dictation.recording-remove-failed", err)
	}
}

// Retry transcribes a saved dictation again. The request is new and billable, so every segment gets a fresh operation
// ID, as the desktop app does. It runs only while the service is idle, and holds the service until it returns, so a
// dictation cannot start over it. The caller decides what to do with the text; the recording is left as it is.
func (s *Service) Retry(ctx context.Context, id string) (string, error) {
	if s.cfg.Recorder == nil || s.cfg.Transcriber == nil {
		return "", ErrNoRecordings
	}
	s.mu.Lock()
	if s.phase != PhaseIdle {
		s.mu.Unlock()
		return "", ErrBusy
	}
	s.phase = PhaseTranscribing
	s.mu.Unlock()
	defer s.endRetry()
	files, err := s.cfg.Recorder.Load(id)
	if err != nil {
		return "", err
	}
	return transcribeFiles(ctx, s.cfg.Transcriber, newRetryOperationID(), false, files)
}

// endRetry returns the service to idle after a retry, unless a dictation has taken it over meanwhile.
func (s *Service) endRetry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == PhaseTranscribing && s.cur == nil && s.starting == nil {
		s.phase = PhaseIdle
	}
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

// Cancel drops the current dictation, or the one still opening: the microphone closes, nothing more is sent, the
// backend's preparation is released as a user cancel, and a Stop that is waiting on the backend returns ErrCancelled.
// Cancel on an idle service does nothing.
func (s *Service) Cancel() {
	s.mu.Lock()
	op, stream := s.cur, platform.MicrophoneStream(nil)
	if op != nil {
		stream = op.stream
	} else {
		op = s.starting
	}
	s.cur, s.starting = nil, nil
	s.phase = PhaseIdle
	s.mu.Unlock()
	if op == nil {
		return
	}
	s.releasePreparation(op, true)
	if stream != nil {
		_ = stream.Close()
	}
	s.publishLevel(0)
}

// newOperation returns a dictation that has not opened its microphone yet.
func newOperation() *operation {
	ctx, cancel := context.WithCancel(context.Background())
	return &operation{id: newOperationID(), ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

// newOperationID returns a random key for one dictation, short enough for the backend's idempotency header.
func newOperationID() string {
	return "dictation-" + randomHex()
}

// newRetryOperationID returns a key for one retry of a saved dictation. It never reuses the failed attempt's key, which
// the backend has already settled.
func newRetryOperationID() string {
	return "dictation-retry-" + randomHex()
}

func randomHex() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}
