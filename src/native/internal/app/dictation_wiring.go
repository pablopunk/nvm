package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	dict "github.com/pablopunk/nvm/native/internal/dictation"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Dictation timings. The indicator stays for a moment after a dictation with no speech, and longer after a failure,
// so the reason can be read before it goes. The clipboard is restored after this delay, once the paste has read it.
const (
	dictationNoSpeechDismiss = 2200 * time.Millisecond
	dictationFailureDismiss  = 4 * time.Second
	dictationRestoreDelay    = 500 * time.Millisecond
)

// dictationTranscriber is the service's backend port. It sends through the account session's client, and reports
// the refusals the account view explains (the free pool's cap) to the session.
type dictationTranscriber struct {
	session *auth.Session
}

// Transcribe sends one WebM file and notes a spend-cap refusal on the account.
func (t dictationTranscriber) Transcribe(ctx context.Context, operationID string, file []byte) (string, error) {
	text, err := t.session.Client().Transcribe(ctx, backend.TranscriptionRequest{OperationID: operationID, Audio: file})
	if err != nil {
		t.session.NoteError(err)
	}
	return text, err
}

// Prepare checks admission for operationID ahead of the recording's end.
func (t dictationTranscriber) Prepare(ctx context.Context, operationID string) error {
	return t.session.Client().PrepareTranscription(ctx, operationID)
}

// dictationService returns the app's dictation service, created on first use from the platform ports. It runs on
// the UI thread, so the lazy creation needs no lock.
func (p *App) dictationService() *dict.Service {
	if p.dictation == nil {
		p.dictation = dict.NewService(dict.Config{
			Microphone:  p.ports.Microphone,
			Codec:       p.ports.Opus,
			Transcriber: dictationTranscriber{session: p.account},
			OnLevel:     p.dictationLevel,
		})
	}
	return p.dictation
}

// toggleDictation starts a dictation when none runs, and ends the running one. A dictation that is transcribing
// ignores the key; the next press starts a new one once its text is pasted.
func (p *App) toggleDictation() {
	switch p.dictationService().Snapshot().Phase {
	case dict.PhaseIdle:
		p.startDictation()
	case dict.PhaseRecording:
		p.stopDictation()
	}
}

// startDictation opens the microphone off the UI thread. The indicator says it waits for the microphone until the
// stream is open, then that it listens.
func (p *App) startDictation() {
	if !p.dictationCanStart() {
		p.endDictationWithMessage("Sign in to use cloud dictation.")
		return
	}
	p.dictationEpoch++
	p.setDictation(palindicator.State{Mode: palindicator.ModeWaitingForMicrophone})
	service := p.dictationService()
	safego.Go("dictation.start", func(context.Context) {
		err := service.Start("")
		p.redraw(func() {
			if err != nil {
				p.failDictation(err)
				return
			}
			p.setDictation(palindicator.State{Mode: palindicator.ModeListening})
		})
	})
}

// dictationCanStart reports whether a signed-in account and the microphone and Opus ports are available.
func (p *App) dictationCanStart() bool {
	return p.account != nil && p.account.Snapshot().Connected && p.ports.Microphone != nil && p.ports.Opus != nil
}

// stopDictation ends the recording, transcribes it off the UI thread and pastes the text into the app that had focus.
func (p *App) stopDictation() {
	p.setDictation(palindicator.State{Mode: palindicator.ModeTranscribing})
	service := p.dictationService()
	safego.Go("dictation.stop", func(context.Context) {
		text, err := service.Stop()
		if err != nil {
			p.redraw(func() { p.failDictation(err) })
			return
		}
		if text == "" {
			p.redraw(p.endDictationWithoutSpeech)
			return
		}
		p.redraw(p.hideDictation)
		p.insertDictation(text)
	})
}

// failDictation shows why a dictation ended without text. A cancelled dictation has no message and just hides.
func (p *App) failDictation(err error) {
	if p.account != nil {
		p.account.NoteError(err)
	}
	message := dict.FailureMessage(err)
	if message == "" {
		p.hideDictation()
		return
	}
	p.endDictationWithMessage(message)
}

// endDictationWithMessage shows a failure and hides the indicator after the failure timing.
func (p *App) endDictationWithMessage(message string) {
	p.dictationEpoch++
	p.setDictation(palindicator.State{Mode: palindicator.ModeFailed, Detail: message})
	p.hideDictationAfter(p.dictationEpoch, dictationFailureDismiss)
}

// endDictationWithoutSpeech shows that the recording held no speech, then hides the indicator.
func (p *App) endDictationWithoutSpeech() {
	p.dictationEpoch++
	p.setDictation(palindicator.State{Mode: palindicator.ModeNoSpeech})
	p.hideDictationAfter(p.dictationEpoch, dictationNoSpeechDismiss)
}

// insertDictation puts the text on the clipboard, sends the paste gesture to the app that had focus, and restores
// the previous clipboard text once the paste has had time to read it. A paste the OS refuses leaves the text on the
// clipboard and says so.
func (p *App) insertDictation(text string) {
	clipboard := p.ports.Clipboard
	previous, _ := clipboard.ReadText()
	if err := clipboard.WriteText(text); err != nil {
		p.toastFromBackground("Dictation could not reach the clipboard.")
		return
	}
	p.notePaletteHidden()
	safego.Go("dictation.paste", func(context.Context) {
		if err := p.ports.Paster.PasteIntoFrontmost(""); err != nil {
			p.toastFromBackground(pasteFailureMessage(err))
			return
		}
		time.Sleep(dictationRestoreDelay)
		restoreDictationClipboard(clipboard, text, previous)
	})
}

// restoreDictationClipboard puts the previous text back only when the clipboard still holds the dictation, so a copy
// the user made meanwhile is kept.
func restoreDictationClipboard(clipboard platform.Clipboard, dictated, previous string) {
	if current, err := clipboard.ReadText(); err == nil && current == dictated {
		_ = clipboard.WriteText(previous)
	}
}

// dictationLevel receives the meter level from the capture goroutine and draws it while the dictation listens.
func (p *App) dictationLevel(level float64) {
	p.redraw(func() {
		if p.dictationView.Mode == palindicator.ModeListening {
			p.setDictation(palindicator.State{Mode: palindicator.ModeListening, Level: level})
		}
	})
}
