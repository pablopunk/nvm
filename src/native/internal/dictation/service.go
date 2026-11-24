package dictation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	// ErrNoTranscriber is a dictation with audio and no backend to send it to; the audio is kept.
	ErrNoTranscriber = errors.New("dictation has no transcription service")
)

// defaultReadyTimeout bounds the wait for a Bluetooth microphone to start picking up sound.
const defaultReadyTimeout = 7 * time.Second

// Config holds the ports a Service runs on. OnLevel receives the meter level about every 100 ms while recording,
// from the capture goroutine. Deferred consumers must check current before applying it. Background runs the backend calls that
// nobody waits for (preparation and its cancellation); nil runs them on a supervised goroutine. Warn logs a
// recovery the user does not see, such as the default microphone standing in for a chosen one that failed.
type Config struct {
	Microphone  platform.Microphone
	Codec       platform.OpusCodec
	Transcriber Transcriber
	OnLevel     func(level float64, current func() bool)
	Background  func(name string, fn func())
	Warn        func(event string, fields map[string]any)
	// Recorder keeps the audio on disk from the first frame until the text is in. Nil keeps no audio, so nothing can
	// be retried.
	Recorder Recorder
	// Retry bounds the automatic retries of every step that fails for a passing reason: opening the microphone, the
	// readiness wait, reopening a stream that failed mid-recording and each transcription. The zero value is the default.
	Retry RetryPolicy
	// Clock times the capture, to log a dictation whose audio is shorter than the time it was held; nil skips the check.
	Clock platform.Clock
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

// ActiveRecordingID names the dictation that is recording or being transcribed now, or "" when there is none.
func (s *Service) ActiveRecordingID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil {
		return ""
	}
	return s.cur.id
}

// Snapshot returns the phase and the latest level.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	phase := s.phase
	s.mu.Unlock()
	return Snapshot{Phase: phase, Level: math.Float64frombits(s.level.Load())}
}

// Start prepares the backend, opens deviceID (or the default device when it is "") and starts capturing. It returns
// after capture receives a frame, so a missing or refused microphone is reported here and not later. Opening is retried
// under the retry policy while it fails for a passing reason. A Bluetooth microphone also has to start delivering sound first
// (ErrNotReady when it does not), so its first words are not lost. A Cancel while the microphone is still opening ends
// the start with ErrCancelled, and the stream that opened late is closed.
func (s *Service) Start(deviceID string) error {
	return s.StartWithLevel(deviceID, nil)
}

// StartWithLevel binds a host-owned meter to the admitted recording only.
func (s *Service) StartWithLevel(deviceID string, sink func(float64, string, func() bool)) error {
	return s.StartOn(s.cfg.Transcriber, deviceID, sink)
}

// StartOn is StartWithLevel with the transcriber this dictation's segments go to, such as the on-device model.
func (s *Service) StartOn(transcriber Transcriber, deviceID string, sink func(float64, string, func() bool)) error {
	s.mu.Lock()
	if s.phase != PhaseIdle {
		s.mu.Unlock()
		return ErrBusy
	}
	s.phase = PhaseRecording
	s.level.Store(0)
	op := newOperation()
	op.transcriber = transcriber
	op.levelSink = sink
	op.onLevel = func(level float64) { s.publishOperationLevel(op, level) }
	op.retry = s.cfg.Retry
	op.onRetry = s.retryWarning("dictation.capture-retry")
	op.reopen = func(ctx context.Context) (platform.MicrophoneStream, error) { return s.openMicrophone(ctx, deviceID) }
	s.starting = op
	s.mu.Unlock()

	op.ready = make(chan struct{})
	if s.needsReadiness(op.ctx, deviceID) {
		op.tracker = &readinessTracker{}
	}
	s.prepare(op)
	stream, encoder, err := s.openCaptureWithRetry(op, deviceID)

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
	if s.cfg.Recorder != nil {
		op.journal = newJournal(s.cfg.Recorder, op.id, op.track(), s.warn)
	}
	if op.transcriber != nil && !readsWholeRecording(op.transcriber) {
		op.live = newLiveTranscription(s, op)
	}
	s.cur = op
	if s.cfg.Clock != nil {
		op.capturing = s.cfg.Clock.Now()
	}
	safego.Go("dictation.capture", func(context.Context) { op.capture() })
	s.mu.Unlock()
	return s.awaitReady(op)
}

// retryWarning returns the hook that logs each automatic retry of one step, so a failure the user never sees still
// names the call that failed.
func (s *Service) retryWarning(event string) func(attempt int, err error) {
	return func(attempt int, err error) {
		if s.cfg.Warn != nil {
			s.cfg.Warn(event, map[string]any{"error": err.Error(), "attempt": attempt})
		}
	}
}

// prepare warms the backend's admission while the user speaks. The transcription waits for it to finish, so the
// backend sees the preparation first. A failed preparation is recorded as not prepared and otherwise ignored; the
// transcription request repeats the admission check.
func (s *Service) prepare(op *operation) {
	op.prepared = make(chan struct{})
	if op.transcriber == nil {
		close(op.prepared)
		return
	}
	s.cfg.Background("dictation.prepare", func() {
		defer close(op.prepared)
		op.preparedOK = op.transcriber.Prepare(op.ctx, op.id) == nil
	})
}

// releasePreparation cancels op's context and releases its backend admission once the preparation has finished.
func (s *Service) releasePreparation(op *operation, userCancelled bool) {
	op.cancel()
	if op.transcriber == nil {
		return
	}
	s.cfg.Background("dictation.cancel-preparation", func() {
		<-op.prepared
		_ = op.transcriber.CancelPreparation(context.Background(), op.id, userCancelled)
		op.transcriber.Finished(op.id)
	})
}

// Stop ends the recording, waits for the segments still being transcribed and returns the joined text. An empty text means no
// speech was captured or the speech was silent; the caller shows that as a state, not as an error. Whatever happens,
// audio that holds sound stays in the recorder until its text is in.
func (s *Service) Stop() (string, error) {
	s.mu.Lock()
	op := s.cur
	if s.phase != PhaseRecording || op == nil {
		s.mu.Unlock()
		return "", ErrNotRecording
	}
	s.phase = PhaseTranscribing
	s.level.Store(0)
	s.mu.Unlock()

	op.endCapture()
	<-op.done
	s.checkCaptureCovered(op)
	text, err := s.finish(op)
	if err != nil && !errors.Is(err, ErrCancelled) || !op.hasSpeechAudio() {
		s.releasePreparation(op, false)
	}
	op.cancel()
	s.release(op)
	if op.transcriber != nil {
		op.transcriber.Finished(op.id)
	}
	return text, err
}

// finish turns a captured operation into text. The recording is on disk before anything is sent, and it stays there on
// every failure: a microphone that failed mid-recording, no backend, a cancellation, or a transcription that failed
// after its retries.
func (s *Service) finish(op *operation) (string, error) {
	defer func() { _ = op.encoder.Close() }()
	saved := op.journal.finish(op.segments)
	if !op.hasSpeechAudio() {
		return "", op.failure
	}
	if op.failure != nil {
		return "", s.keepFailed(op, saved, op.failure)
	}
	if op.transcriber == nil {
		return "", s.keepFailed(op, saved, ErrNoTranscriber)
	}
	text, err := s.transcribe(op)
	if op.ctx.Err() != nil {
		s.keepCancelled(op, saved)
		return "", ErrCancelled
	}
	if err != nil {
		return "", s.keepFailed(op, saved, err)
	}
	s.discardRecording(op.id)
	return text, nil
}

// Retry transcribes a saved dictation again. The request is new and billable, so every segment gets a fresh operation
// ID, as the desktop app does. It runs only while the service is idle, and holds the service until it returns, so a
// dictation cannot start over it. The caller decides what to do with the text; the recording is left as it is.
func (s *Service) Retry(ctx context.Context, id string) (string, error) {
	return s.RetryOn(ctx, id, s.cfg.Transcriber)
}

// RetryOn is Retry with the transcriber the saved dictation goes to, such as the on-device model.
func (s *Service) RetryOn(ctx context.Context, id string, transcriber Transcriber) (string, error) {
	if s.cfg.Recorder == nil || transcriber == nil {
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
	return s.retryRecording(ctx, id, files, transcriber)
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

// newOperation returns a dictation that has not opened its microphone yet.
func newOperation() *operation {
	ctx, cancel := context.WithCancel(context.Background())
	captureCtx, stopCapture := context.WithCancel(ctx)
	return &operation{
		id: newOperationID(), ctx: ctx, cancel: cancel, captureCtx: captureCtx, stopCapture: stopCapture,
		done: make(chan struct{}),
	}
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

// transcribe returns the text of a captured dictation: the live pieces once the last one is answered, or the whole
// recording in one pass for a transcriber that reads it whole.
func (s *Service) transcribe(op *operation) (string, error) {
	if op.live != nil {
		op.sendLive()
		return op.live.wait(len(op.segments))
	}
	select {
	case <-op.prepared:
	case <-op.ctx.Done():
		return "", op.ctx.Err()
	}
	passes := wholeRecordingPasses(op.track(), op.segments)
	return transcribePasses(op.ctx, op.transcriber, s.cfg.Retry, op.id, op.preparedOK, passes, s.retryWarning("dictation.transcribe-retry"))
}

// captureGapLimit is how much shorter than the time the user held a dictation its audio may be before the gap is logged.
const captureGapLimit = time.Second

// checkCaptureCovered logs a dictation whose audio is shorter than the time between its start and Stop, which means
// the microphone stopped delivering partway and the words after that were never heard.
func (s *Service) checkCaptureCovered(op *operation) {
	if s.cfg.Clock == nil {
		return
	}
	held := s.cfg.Clock.Now().Sub(op.capturing)
	audio := time.Duration(op.frames*FrameMillis) * time.Millisecond
	if held-audio > captureGapLimit && s.cfg.Warn != nil {
		s.cfg.Warn("dictation.capture-gap", map[string]any{"heldMs": held.Milliseconds(), "audioMs": audio.Milliseconds(), "id": op.id})
	}
}
