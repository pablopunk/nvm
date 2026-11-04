package app

import (
	"time"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Dictation timings: the indicator stays a moment after a dictation with no speech, and longer after a failure, so
// the reason can be read. The clipboard is restored after this delay, once the paste has read it.
const (
	dictationNoSpeechDismiss = 2200 * time.Millisecond
	dictationFailureDismiss  = 4 * time.Second
	dictationRestoreDelay    = 500 * time.Millisecond
	dictationSignInMessage   = "Sign in to use cloud dictation."
)

// dictationAccount is the part of the account the flow needs: whether a dictation may start, the transcription port,
// and the account notice for a refusal.
type dictationAccount interface {
	SignedIn() bool
	Transcriber() dict.Transcriber
	NoteError(err error)
}

// dictationPanel draws the indicator. Show sets its state and makes it visible without taking the keyboard; Hide
// closes it. Both run on the UI thread.
type dictationPanel interface {
	Show(state palindicator.State)
	Hide()
}

// dictationDeps are the ports and callbacks a dictationFlow runs on. Dispatch runs a function on the UI thread,
// Background runs one off it, and After schedules a function on the UI thread. Tests supply inline versions, so
// every step of a dictation is deterministic.
type dictationDeps struct {
	Microphone    platform.Microphone
	Codec         platform.OpusCodec
	Clipboard     platform.Clipboard
	Paster        platform.Paster
	Account       dictationAccount
	Settings      func() dictationSettings
	Panel         dictationPanel
	Dispatch      func(fn func())
	Background    func(name string, fn func())
	After         func(d time.Duration, fn func())
	PaletteHidden func()
	Toast         func(msg string)
	RestoreDelay  time.Duration
	OnDevices     func()
}

// dictationFlow connects the dictation service to the palette: the Dictate command starts and stops it, the
// indicator shows each state, the text is pasted into the app that had focus, and failures are explained. Every
// change to the indicator runs on the UI thread through Dispatch.
type dictationFlow struct {
	deps    dictationDeps
	service *dict.Service
	state   palindicator.State
	// open reports whether the panel is showing state. A late level from a cancelled capture must not reopen it.
	open    bool
	epoch   int
	devices []platform.MicrophoneDevice
}

// newDictationFlow returns an idle flow over deps.
func newDictationFlow(deps dictationDeps) *dictationFlow {
	f := &dictationFlow{deps: deps}
	f.service = dict.NewService(dict.Config{
		Microphone:  deps.Microphone,
		Codec:       deps.Codec,
		Transcriber: deps.Account.Transcriber(),
		OnLevel:     f.onLevel,
		Background:  deps.Background,
	})
	return f
}

// Toggle starts a dictation when none runs and ends the running one. A dictation that is transcribing ignores the key.
func (f *dictationFlow) Toggle() {
	switch f.service.Snapshot().Phase {
	case dict.PhaseIdle:
		f.Start()
	case dict.PhaseRecording:
		f.Stop()
	}
}

// Start opens the microphone off the UI thread. The indicator waits for the microphone until the stream is open.
func (f *dictationFlow) Start() {
	if !f.deps.Account.SignedIn() {
		f.failWith(dictationSignInMessage)
		return
	}
	f.epoch++
	f.show(palindicator.State{Mode: palindicator.ModeWaitingForMicrophone})
	deviceID := f.deps.Settings().DeviceID
	f.deps.Background("dictation.start", func() {
		err := f.service.Start(deviceID)
		f.deps.Dispatch(func() {
			if err != nil {
				f.fail(err)
				return
			}
			if f.open && f.state.Mode == palindicator.ModeWaitingForMicrophone {
				f.show(palindicator.State{Mode: palindicator.ModeListening})
			}
		})
	})
}

// Stop ends the recording, transcribes it off the UI thread and pastes the text.
func (f *dictationFlow) Stop() {
	f.show(palindicator.State{Mode: palindicator.ModeTranscribing})
	f.deps.Background("dictation.stop", func() {
		text, err := f.service.Stop()
		if err != nil {
			f.deps.Dispatch(func() { f.fail(err) })
			return
		}
		if text == "" {
			f.deps.Dispatch(f.noSpeech)
			return
		}
		f.deps.Dispatch(f.hide)
		f.insert(text)
	})
}

// Cancel drops the running dictation, releases its backend preparation as a user cancel, and closes the indicator. A
// Stop that is transcribing returns the cancellation and closes the indicator itself.
func (f *dictationFlow) Cancel() {
	if f.service.Snapshot().Phase == dict.PhaseIdle {
		return
	}
	f.service.Cancel()
	f.hide()
}

// RefreshDevices lists the microphones in the background and reports them through OnDevices.
func (f *dictationFlow) RefreshDevices() {
	f.deps.Background("dictation.devices", func() {
		devices, err := f.deps.Microphone.Devices()
		if err != nil {
			devices = nil
		}
		f.deps.Dispatch(func() {
			f.devices = devices
			if f.deps.OnDevices != nil {
				f.deps.OnDevices()
			}
		})
	})
}

// Devices returns the last listed microphones. It runs on the UI thread and never blocks.
func (f *dictationFlow) Devices() []platform.MicrophoneDevice {
	return f.devices
}

// onLevel receives the meter level from the capture goroutine and draws it. A level means the microphone is live, so
// it counts as listening even before Start has finished showing that.
func (f *dictationFlow) onLevel(level float64) {
	f.deps.Dispatch(func() {
		if f.open && (f.state.Mode == palindicator.ModeListening || f.state.Mode == palindicator.ModeWaitingForMicrophone) {
			f.show(palindicator.State{Mode: palindicator.ModeListening, Level: level})
		}
	})
}

// fail explains a failed dictation. A cancelled one has no message and just closes the indicator. The account notes
// a refusal from the backend, so the account view can explain the free pool's cap.
func (f *dictationFlow) fail(err error) {
	f.deps.Account.NoteError(err)
	message := dict.FailureMessage(err)
	if message == "" {
		f.hide()
		return
	}
	f.failWith(message)
}

func (f *dictationFlow) failWith(message string) {
	f.epoch++
	f.show(palindicator.State{Mode: palindicator.ModeFailed, Detail: message})
	f.hideAfter(f.epoch, dictationFailureDismiss)
}

func (f *dictationFlow) noSpeech() {
	f.epoch++
	f.show(palindicator.State{Mode: palindicator.ModeNoSpeech})
	f.hideAfter(f.epoch, dictationNoSpeechDismiss)
}

// hideAfter closes the indicator after d, unless another dictation began in the meantime.
func (f *dictationFlow) hideAfter(epoch int, d time.Duration) {
	f.deps.After(d, func() {
		if epoch == f.epoch {
			f.hide()
		}
	})
}

func (f *dictationFlow) show(state palindicator.State) {
	f.state, f.open = state, true
	f.deps.Panel.Show(state)
}

func (f *dictationFlow) hide() {
	f.state, f.open = palindicator.State{}, false
	f.deps.Panel.Hide()
}

// insert places the text where the user is typing. The text goes on the clipboard marked as concealed, so Clipboard
// History never records it, and is pasted after the palette hides. Unless the settings ask to keep the text, the
// previous clipboard text comes back once the paste has read it. A refused paste leaves the text for a manual paste.
func (f *dictationFlow) insert(text string) {
	settings := f.deps.Settings()
	clipboard := f.deps.Clipboard
	previous, _ := clipboard.ReadText()
	if err := writeDictation(clipboard, text, settings.CopyToClipboard); err != nil {
		f.deps.Toast(dictationClipboardMessage(err))
		return
	}
	f.deps.PaletteHidden()
	f.deps.Background("dictation.paste", func() {
		if err := f.deps.Paster.PasteIntoFrontmost(""); err != nil {
			f.deps.Toast(pasteFailureMessage(err))
			return
		}
		if !settings.CopyToClipboard {
			time.Sleep(f.deps.RestoreDelay)
			restoreDictationClipboard(clipboard, text, previous)
		}
	})
}

// writeDictation puts text on the clipboard: concealed unless the user asked to keep it there.
func writeDictation(clipboard platform.Clipboard, text string, keep bool) error {
	if keep {
		return clipboard.WriteText(text)
	}
	concealed, ok := clipboard.(platform.ConcealedClipboard)
	if !ok {
		return platform.ErrUnsupported
	}
	return concealed.WriteConcealedText(text)
}

// dictationClipboardMessage says why the text could not be placed on the clipboard.
func dictationClipboardMessage(err error) string {
	if err == platform.ErrUnsupported {
		return "Dictation needs a clipboard that keeps text out of history, which this system does not offer yet."
	}
	return "Dictation could not reach the clipboard."
}

// restoreDictationClipboard puts the previous text back only when the clipboard still holds the dictation, so a copy
// the user made meanwhile is kept.
func restoreDictationClipboard(clipboard platform.Clipboard, dictated, previous string) {
	if current, err := clipboard.ReadText(); err == nil && current == dictated {
		_ = clipboard.WriteText(previous)
	}
}
