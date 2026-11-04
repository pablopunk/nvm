package app

import (
	"context"
	"os"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/cleanup"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// dictationTranscriber is the service's backend port. It sends through the account session's client.
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

// appDictationAccount adapts the app's account session to the flow's account port.
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

// dictationFlowFor returns the app's dictation flow, created on first use from the platform ports. It runs on the UI
// thread, so the lazy creation needs no lock.
func (p *App) dictationFlowFor() *dictationFlow {
	if p.dictation == nil {
		p.dictation = newDictationFlow(p.dictationDeps())
	}
	return p.dictation
}

// dictationDeps wires the flow to the app: UI work goes through the palette's redraw, background work to supervised
// goroutines, and timers fire back on the UI thread.
func (p *App) dictationDeps() dictationDeps {
	return dictationDeps{
		Microphone:    p.ports.Microphone,
		Codec:         p.ports.Opus,
		Clipboard:     p.ports.Clipboard,
		Paster:        p.ports.Paster,
		Account:       appDictationAccount{app: p},
		Settings:      p.dictationSettings,
		Panel:         &dictationWindow{app: p},
		Dispatch:      p.redraw,
		Background:    supervisedDictation,
		After:         p.dictationAfter,
		PaletteHidden: p.notePaletteHidden,
		Toast:         p.toastFromBackground,
		Warn: func(event string, fields map[string]any) {
			logx.Default().Warn(event, logx.F(fields))
		},
		OpenMicrophoneSettings: func() error {
			return p.ports.System.OpenSettings(systemfeat.PrivacyPaneID, "Microphone")
		},
		Complete:     p.dictationCompleter(),
		ScreenText:   p.dictationScreenText,
		RestoreDelay: dictationRestoreDelay,
		OnDevices:    p.Refresh,
	}
}

func supervisedDictation(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() })
}

// dictationAfter runs fn on the UI thread after d.
func (p *App) dictationAfter(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { p.redraw(fn) })
}

// dictationCompleter answers the AI cleanup prompt on the fast model through the account's client. A signed-out
// account is refused before any text is sent.
func (p *App) dictationCompleter() cleanup.Completer {
	return func(ctx context.Context, system, user string) (string, error) {
		if p.account == nil || !p.account.Snapshot().Connected {
			return "", chat.ErrNotSignedIn
		}
		return chat.Complete(ctx, backendCompletions{client: p.account.Client()}, system, user)
	}
}

// backendCompletions adapts the backend client to the one-shot completion's source.
type backendCompletions struct {
	client *backend.Client
}

// ActiveModel reads the route for the query.
func (b backendCompletions) ActiveModel(ctx context.Context, query backend.ActiveModelQuery) (backend.ModelDescriptor, error) {
	return b.client.ActiveModel(ctx, query)
}

// ChatStream opens the streamed reply for req.
func (b backendCompletions) ChatStream(ctx context.Context, req backend.ChatRequest) (chat.EventStream, error) {
	stream, err := b.client.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// dictationScreenText reads the text the screen shows, through the same OCR service the extensions use.
func (p *App) dictationScreenText(ctx context.Context) (string, error) {
	service := ocr.New(ocr.Deps{Text: p.ports.Text, Capture: p.ports.Capture, FS: p.ports.FS, TempDir: os.TempDir()})
	result, err := service.Screen(ctx, ocr.Options{})
	return result.Text, err
}
