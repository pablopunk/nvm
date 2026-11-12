package dictation

import (
	"context"
	"errors"
	"sync"

	"github.com/pablopunk/nvm/native/internal/dictation/audiomute"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Account is the part of the account a dictation needs: whether hosted transcription is available, the backend port,
// and the note a refusal leaves for the account view.
type Account interface {
	SignedIn() bool
	Transcriber() Transcriber
	NoteError(err error)
}

// EngineConfig holds the ports an Engine runs on. Recordings keeps the audio of unfinished dictations; nil keeps none.
// Muter silences the output while a dictation records; nil leaves the output alone. Background and Warn are as in Config.
type EngineConfig struct {
	Microphone platform.Microphone
	Codec      platform.OpusCodec
	Account    Account
	Recordings *recordings.Store
	Retry      RetryPolicy
	Muter      *audiomute.Muter
	OnLevel    func(level float64, current func() bool)
	Background func(name string, fn func())
	Warn       func(event string, fields map[string]any)
}

// Engine is the host's dictation for the extensions: the one dictation the service runs, the output mute around it,
// and the saved recordings. Audio stays in the host; the caller sees transcripts and the failure words the user reads.
type Engine struct {
	cfg     EngineConfig
	service *Service
	muteMu  sync.Mutex
	lease   *audiomute.Lease
}

// NewEngine returns an idle engine over cfg.
func NewEngine(cfg EngineConfig) *Engine {
	engine := &Engine{cfg: cfg}
	serviceConfig := Config{
		Microphone: cfg.Microphone,
		Codec:      cfg.Codec,
		OnLevel:    cfg.OnLevel,
		Background: cfg.Background,
		Warn:       cfg.Warn,
		Retry:      cfg.Retry,
	}
	if cfg.Account != nil {
		serviceConfig.Transcriber = cfg.Account.Transcriber()
	}
	if cfg.Recordings != nil {
		serviceConfig.Recorder = cfg.Recordings
	}
	engine.service = NewService(serviceConfig)
	return engine
}

// RecoverMute gives the output back its state when the last run stopped in the middle of a dictation.
func (e *Engine) RecoverMute() error {
	if e.cfg.Muter == nil {
		return nil
	}
	return e.cfg.Muter.RecoverAfterCrash()
}

// Status maps the service's phase onto the extension API's status.
func (e *Engine) Status() api.DictationStatus {
	switch e.service.Snapshot().Phase {
	case PhaseRecording:
		return api.DictationRecording
	case PhaseTranscribing:
		return api.DictationTranscribing
	}
	return api.DictationIdle
}

// APIAvailable reports whether the account can send audio for hosted transcription.
func (e *Engine) APIAvailable() bool {
	return e.cfg.Account != nil && e.cfg.Account.SignedIn()
}

// Devices lists the microphones, the default one marked. A list that cannot be read is an empty one.
func (e *Engine) Devices() ([]api.DictationDevice, error) {
	devices, err := e.cfg.Microphone.Devices()
	if err != nil {
		return nil, userFailure(err)
	}
	out := make([]api.DictationDevice, 0, len(devices))
	for _, device := range devices {
		out = append(out, api.DictationDevice{ID: device.ID, Title: device.Title, IsDefault: device.Default})
	}
	return out, nil
}

// Start mutes the output when asked, then opens the microphone. A failed start gives the output back.
func (e *Engine) Start(options api.DictationStartOptions) error {
	return e.StartWithLevel(options, nil)
}

// StartWithLevel keeps the meter bound to the recording admitted by this start.
func (e *Engine) StartWithLevel(options api.DictationStartOptions, sink func(float64, string, func() bool)) error {
	if options.MuteSystemAudioWhileRecord {
		e.beginMute()
	}
	if err := e.service.StartWithLevel(microphoneID(options.DeviceID), sink); err != nil {
		e.restoreMute()
		return userFailure(err)
	}
	return nil
}

// Stop gives the output back, then ends the recording and returns the hosted transcript.
func (e *Engine) Stop() (string, error) {
	e.restoreMute()
	text, err := e.service.Stop()
	if err != nil {
		return "", userFailure(err)
	}
	return text, nil
}

// Cancel drops the dictation in progress and gives the output back.
func (e *Engine) Cancel() error {
	e.service.Cancel()
	e.restoreMute()
	return nil
}

// Recordings lists the saved dictations, newest first.
func (e *Engine) Recordings() ([]api.SavedDictation, error) {
	if e.cfg.Recordings == nil {
		return nil, nil
	}
	saved, err := e.cfg.Recordings.List()
	if err != nil {
		return nil, userFailure(err)
	}
	out := make([]api.SavedDictation, 0, len(saved))
	for _, recording := range saved {
		out = append(out, api.SavedDictation{ID: recording.ID, CreatedAtMs: recording.CreatedAt, SegmentCount: recording.SegmentCount, Failure: recording.Failure})
	}
	return out, nil
}

// Retry transcribes a saved dictation again. The recording stays until the caller deletes it.
func (e *Engine) Retry(ctx context.Context, id string) (string, error) {
	text, err := e.service.Retry(ctx, id)
	if err != nil {
		return "", userFailure(err)
	}
	return text, nil
}

// DeleteRecording drops a saved dictation's audio.
func (e *Engine) DeleteRecording(id string) error {
	if e.cfg.Recordings == nil {
		return api.ErrNotFound
	}
	if err := e.cfg.Recordings.Remove(id); err != nil {
		return userFailure(err)
	}
	return nil
}

// beginMute silences the output for the dictation. A mute that fails is logged, and the dictation goes on audible.
func (e *Engine) beginMute() {
	if e.cfg.Muter == nil {
		return
	}
	lease, err := e.cfg.Muter.Begin()
	if err != nil {
		e.warn("dictation.mute-failed", err)
		return
	}
	e.muteMu.Lock()
	defer e.muteMu.Unlock()
	e.lease = lease
}

// restoreMute gives the output back its state from before the dictation. Only the first call does anything.
func (e *Engine) restoreMute() {
	e.muteMu.Lock()
	lease := e.lease
	e.lease = nil
	e.muteMu.Unlock()
	if lease == nil {
		return
	}
	if err := lease.Restore(); err != nil {
		e.warn("dictation.unmute-failed", err)
	}
}

func (e *Engine) warn(event string, err error) {
	if e.cfg.Warn != nil {
		e.cfg.Warn(event, map[string]any{"error": err.Error()})
	}
}

// microphoneID maps the extension API's default-device spellings onto the platform's: "" and "default" are the
// system default microphone.
func microphoneID(id string) string {
	if id == "default" {
		return ""
	}
	return id
}

// userError carries the words the user reads as its message, and the API cause that classifies it for extensions.
// A cancelled dictation has an empty message: the caller closes its indicator without saying anything.
type userError struct {
	message string
	cause   error
}

func (e userError) Error() string { return e.message }

func (e userError) Unwrap() error { return e.cause }

// userFailure turns a dictation error into the user's words and the API cause. Cancellation has no words.
func userFailure(err error) error {
	message := FailureMessage(err)
	return userError{message: message, cause: apiCauseOf(err)}
}

func apiCauseOf(err error) error {
	switch {
	case errors.Is(err, ErrCancelled):
		return api.ErrNotAvailable
	case errors.Is(err, platform.ErrDenied):
		return api.ErrPermission
	case errors.Is(err, platform.ErrNotFound):
		return api.ErrNotFound
	}
	return api.ErrNotAvailable
}
