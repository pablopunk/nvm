package app

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	dictbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// dictationTranscriber is the engine's backend port. It sends through the account session's client.
type dictationTranscriber struct {
	session *auth.Session
}

// Transcribe sends one WebM segment.
func (t dictationTranscriber) Transcribe(ctx context.Context, segment dict.Segment) (string, error) {
	return t.session.Client().Transcribe(ctx, backend.TranscriptionRequest{
		OperationID:    segment.OperationID,
		CancellationID: segment.CancellationID,
		Prepared:       segment.Prepared,
		Audio:          segment.File,
	})
}

// Prepare checks admission for operationID ahead of the recording's end.
func (t dictationTranscriber) Prepare(ctx context.Context, operationID string) error {
	return t.session.Client().PrepareTranscription(ctx, operationID)
}

// CancelPreparation releases the admission for operationID.
func (t dictationTranscriber) CancelPreparation(ctx context.Context, operationID string, userCancelled bool) error {
	return t.session.Client().CancelTranscriptionPreparation(ctx, operationID, userCancelled)
}

// appDictationAccount adapts the app's account session to the engine's account port.
type appDictationAccount struct {
	app *App
}

// SignedIn reports whether the account session is connected to a backend.
func (a appDictationAccount) SignedIn() bool {
	return a.app.account != nil && a.app.account.Snapshot().Connected
}

// Transcriber returns the backend port over the account session.
func (a appDictationAccount) Transcriber() dict.Transcriber {
	return dictationTranscriber{session: a.app.account}
}

// NoteError records a spend-cap refusal on the account so the account view explains it.
func (a appDictationAccount) NoteError(err error) {
	if a.app.account != nil {
		a.app.account.NoteError(err)
	}
}

// extensionDictation is the dictation engine the extensions reach. It is created on the first call, when the account
// session exists, and every call goes to that one engine.
type extensionDictation struct {
	app    *App
	once   sync.Once
	engine *dict.Engine
}

// engineFor returns the app's dictation engine, created once.
func (d *extensionDictation) engineFor() *dict.Engine {
	d.once.Do(func() { d.engine = d.app.newDictationEngine() })
	return d.engine
}

// newDictationEngine wires the engine to the platform ports, the account and the app's data folder.
func (p *App) newDictationEngine() *dict.Engine {
	return dict.NewEngine(dict.EngineConfig{
		Microphone: p.ports.Microphone,
		Codec:      p.ports.Opus,
		Account:    appDictationAccount{app: p},
		Recordings: p.dictationRecordingStore(),
		Muter:      p.dictationMuter(),
		Background: supervisedDictation,
		Warn: func(event string, fields map[string]any) {
			logx.Default().Warn(event, logx.F(fields))
		},
	})
}

// dictationEngine returns the app's dictation for the extensions and the startup recovery, created on first use.
func (p *App) dictationEngine() *extensionDictation {
	if p.dictation == nil {
		p.dictation = &extensionDictation{app: p}
	}
	return p.dictation
}

// recoverDictationMute gives the output back its state when the last run stopped in the middle of a dictation.
func (p *App) recoverDictationMute() {
	if err := p.dictationEngine().engineFor().RecoverMute(); err != nil {
		logx.Default().Warn("dictation.unmute-recovery-failed", logx.F{"error": err.Error()})
	}
}

func (d *extensionDictation) Status() api.DictationStatus {
	return d.engineFor().Status()
}

func (d *extensionDictation) APIAvailable() bool {
	return d.engineFor().APIAvailable()
}

func (d *extensionDictation) Devices() ([]api.DictationDevice, error) {
	return d.engineFor().Devices()
}

func (d *extensionDictation) Start(options api.DictationStartOptions) error {
	return d.engineFor().Start(options)
}

func (d *extensionDictation) Stop() (string, error) {
	return d.engineFor().Stop()
}

func (d *extensionDictation) Cancel() error {
	return d.engineFor().Cancel()
}

func (d *extensionDictation) Recordings() ([]api.SavedDictation, error) {
	return d.engineFor().Recordings()
}

func (d *extensionDictation) Retry(ctx context.Context, id string) (string, error) {
	return d.engineFor().Retry(ctx, id)
}

func (d *extensionDictation) DeleteRecording(id string) error {
	return d.engineFor().DeleteRecording(id)
}

func supervisedDictation(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() })
}

// dictationRecordingStore returns the store of unfinished dictations, or nil when the data folder is read only.
func (p *App) dictationRecordingStore() *recordings.Store {
	if p.userReadOnly {
		return nil
	}
	return recordings.New(p.ports.FS, dictationRecordingsDir(), p.ports.Clock.Now)
}

// dictationSettingsAction opens the Dictation Settings form of the dictation built-in, as the Dictate row's menu does.
func (p *App) dictationSettingsAction() search.Action {
	open := api.Action{Kind: api.ActionRun, Title: "Open", Ref: &api.ActionRef{ID: dictbuiltin.RefOpenSettings}}
	return extview.ActionFor(p.extensionViewDeps(p.extensionManager()), dictbuiltin.ID, open)
}
