package app

import (
	"context"
	"errors"
	"sync"
	"time"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/audiomute"
	"github.com/pablopunk/nvm/native/internal/dictation/cleanup"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Dictation timings: the indicator stays a moment after a dictation with no speech, and longer after a failure, so
// the reason can be read. A failure with a button stays longer still, so the button can be pressed. The clipboard is
// restored after this delay, once the paste has read it.
const (
	dictationNoSpeechDismiss        = 2200 * time.Millisecond
	dictationFailureDismiss         = 4 * time.Second
	dictationControlledDismiss      = 12 * time.Second
	dictationScreenTimeout          = 2 * time.Second
	dictationRestoreDelay           = 500 * time.Millisecond
	dictationSignInMessage          = "Sign in to use cloud dictation."
	dictationSettingsFailureMessage = "Could not open Microphone settings. Open System Settings → Privacy & Security → Microphone."
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
// Background runs one off it, and After schedules a function on the UI thread. Warn writes a log entry, Toast shows a
// one-line message, and OpenMicrophoneSettings opens the OS settings pane. Tests supply inline versions, so every
// step of a dictation is deterministic.
type dictationDeps struct {
	Microphone             platform.Microphone
	Codec                  platform.OpusCodec
	Clipboard              platform.Clipboard
	Paster                 platform.Paster
	Account                dictationAccount
	Settings               func() dictationSettings
	Panel                  dictationPanel
	Dispatch               func(fn func())
	Background             func(name string, fn func())
	After                  func(d time.Duration, fn func())
	PaletteHidden          func()
	Toast                  func(msg string)
	Warn                   func(event string, fields map[string]any)
	OpenMicrophoneSettings func() error
	// Complete answers the AI cleanup prompt on the fast model. It is used only while the account is signed in.
	Complete cleanup.Completer
	// ScreenText reads the text the screen shows, for the screen terms the cleanup prompt may use.
	ScreenText func(ctx context.Context) (string, error)
	// Recorder keeps the audio of each dictation until its text is in. Nil keeps none.
	Recorder dict.Recorder
	// Remember adds a finished dictation's text to the history.
	Remember func(text string)
	// Mute silences the speakers while a dictation records. Nil leaves the output alone.
	Mute         *audiomute.Muter
	RestoreDelay time.Duration
	OnDevices    func()
}

// dictationFlow connects the dictation service to the palette: the Dictate command starts and stops it, the
// indicator shows each state and its buttons, the text is pasted into the app that had the keyboard when the
// dictation started, and failures are explained in a toast and in the log. Every change to the indicator runs on the
// UI thread through Dispatch.
type dictationFlow struct {
	deps    dictationDeps
	service *dict.Service
	state   palindicator.State
	// open reports whether the panel is showing state. A late level from a cancelled capture must not reopen it.
	open  bool
	epoch int
	// target is the app that had the keyboard when the dictation started, "" when it is not known. The text is pasted
	// only while that app is in front.
	target  string
	devices []platform.MicrophoneDevice
	// muteMu guards lease, the output mute of the dictation in progress.
	muteMu sync.Mutex
	lease  *audiomute.Lease
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
		Warn:        deps.Warn,
		Recorder:    deps.Recorder,
	})
	return f
}

// Toggle starts a dictation when none runs and ends the running one. A dictation that is transcribing ignores the key,
// and so does one that is still opening the microphone, since there is nothing to stop yet.
func (f *dictationFlow) Toggle() {
	switch f.service.Snapshot().Phase {
	case dict.PhaseIdle:
		f.Start()
	case dict.PhaseRecording:
		if f.state.Mode == palindicator.ModeWaitingForMicrophone {
			return
		}
		f.Stop()
	}
}

// Start opens the microphone off the UI thread. The indicator waits for the microphone until the stream is open.
func (f *dictationFlow) Start() {
	if !f.deps.Account.SignedIn() {
		f.failWith(dictationSignInMessage, palindicator.ControlNone)
		return
	}
	f.epoch++
	f.show(waitingState())
	deviceID := f.deps.Settings().DeviceID
	f.deps.Background("dictation.start", func() {
		f.target = f.foregroundTarget()
		f.beginMute()
		err := f.service.Start(deviceID)
		if err != nil {
			f.restoreMute()
		}
		f.deps.Dispatch(func() {
			if err != nil {
				f.fail(err)
				return
			}
			if f.open && f.state.Mode == palindicator.ModeWaitingForMicrophone {
				f.show(listeningState(0))
			}
		})
	})
}

// foregroundTarget names the app that has the keyboard now. A name the OS cannot give is not an error for the user, so
// the paste simply goes to whatever app is in front when it is sent; any other failure is logged.
func (f *dictationFlow) foregroundTarget() string {
	target, err := f.deps.Paster.FrontmostAppID()
	if err != nil && !errors.Is(err, platform.ErrUnsupported) {
		f.warn("dictation.target-unknown", err)
	}
	return target
}

// Stop ends the recording, transcribes it off the UI thread, cleans it when the settings ask, and pastes the text. The
// screen is read while the transcript is on its way, so the reading costs no time the user waits for.
func (f *dictationFlow) Stop() {
	f.show(palindicator.State{Mode: palindicator.ModeTranscribing})
	settings := f.deps.Settings()
	screen := f.screenTermsFor(settings)
	f.deps.Background("dictation.stop", func() {
		f.restoreMute()
		transcript, err := f.service.Stop()
		if err != nil {
			f.deps.Dispatch(func() { f.fail(err) })
			return
		}
		if transcript == "" {
			f.deps.Dispatch(f.noSpeech)
			return
		}
		text := f.cleaned(transcript, settings, <-screen)
		f.deps.Dispatch(f.hide)
		f.remember(text)
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
	f.deps.Background("dictation.unmute", f.restoreMute)
}

// beginMute silences the output for the dictation. A mute that fails is logged, and the dictation goes on audible.
func (f *dictationFlow) beginMute() {
	if f.deps.Mute == nil {
		return
	}
	lease, err := f.deps.Mute.Begin()
	if err != nil {
		f.warn("dictation.mute-failed", err)
		return
	}
	f.muteMu.Lock()
	defer f.muteMu.Unlock()
	f.lease = lease
}

// restoreMute gives the output back its state from before the dictation. It runs when the dictation stops, is
// cancelled or fails, and only the first call does anything.
func (f *dictationFlow) restoreMute() {
	f.muteMu.Lock()
	lease := f.lease
	f.lease = nil
	f.muteMu.Unlock()
	if err := lease.Restore(); err != nil {
		f.warn("dictation.unmute-failed", err)
	}
}

// Press runs the indicator button the user pressed.
func (f *dictationFlow) Press(control palindicator.Control) {
	switch control {
	case palindicator.ControlCancel:
		f.Cancel()
	case palindicator.ControlMicrophoneSettings:
		f.openMicrophoneSettings()
	}
}

// openMicrophoneSettings closes the indicator and opens the Microphone pane. A pane that will not open is a toast.
func (f *dictationFlow) openMicrophoneSettings() {
	f.hide()
	if f.deps.OpenMicrophoneSettings == nil {
		return
	}
	if err := f.deps.OpenMicrophoneSettings(); err != nil {
		f.report(err, dictationSettingsFailureMessage)
	}
}

// Recover transcribes a saved dictation again in the background, and reports the text, or why it failed, on the UI
// thread through done. The recording stays saved until the caller removes it.
func (f *dictationFlow) Recover(id string, done func(text string, err error)) {
	if !f.deps.Account.SignedIn() {
		done("", errors.New(dictationSignInMessage))
		return
	}
	f.deps.Background("dictation.recover", func() {
		text, err := f.service.Retry(context.Background(), id)
		f.deps.Dispatch(func() { done(text, err) })
	})
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
			f.show(listeningState(level))
		}
	})
}

// fail explains a failed dictation: a toast and a log entry, and the indicator's message with the button that fixes it,
// if there is one. A cancelled one has no message and just closes the indicator. The account notes a refusal from the
// backend, so the account view can explain the free pool's cap.
func (f *dictationFlow) fail(err error) {
	f.deps.Account.NoteError(err)
	message := dict.FailureMessage(err)
	if message == "" {
		f.hide()
		return
	}
	f.report(err, message)
	f.failWith(message, failureControl(err))
}

// failureControl is the button a failed dictation offers: a refused microphone opens its settings pane.
func failureControl(err error) palindicator.Control {
	if errors.Is(err, platform.ErrDenied) {
		return palindicator.ControlMicrophoneSettings
	}
	return palindicator.ControlNone
}

// report shows a message as a toast and writes the error to the log under the dictation.failed event.
func (f *dictationFlow) report(err error, message string) {
	f.warn("dictation.failed", err)
	f.deps.Toast(message)
}

// warn writes a dictation problem to the log under event. It is safe to call from any goroutine.
func (f *dictationFlow) warn(event string, err error) {
	if f.deps.Warn != nil {
		f.deps.Warn(event, map[string]any{"error": err.Error()})
	}
}

func (f *dictationFlow) failWith(message string, control palindicator.Control) {
	f.epoch++
	f.show(palindicator.State{Mode: palindicator.ModeFailed, Detail: message, Control: control})
	dismiss := dictationFailureDismiss
	if control != palindicator.ControlNone {
		dismiss = dictationControlledDismiss
	}
	f.hideAfter(f.epoch, dismiss)
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

// waitingState is the indicator while the microphone opens: it can still be cancelled.
func waitingState() palindicator.State {
	return palindicator.State{Mode: palindicator.ModeWaitingForMicrophone, Control: palindicator.ControlCancel}
}

// listeningState is the indicator while the microphone records at level: it can be cancelled.
func listeningState(level float64) palindicator.State {
	return palindicator.State{Mode: palindicator.ModeListening, Level: level, Control: palindicator.ControlCancel}
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
// History never records it, and is pasted after the palette hides into the app that started the dictation. Unless the
// settings ask to keep the text, the previous clipboard text comes back once the paste has read it. A refused paste
// leaves the text on the clipboard for a manual paste and says why in a toast.
func (f *dictationFlow) insert(text string) {
	settings := f.deps.Settings()
	clipboard := f.deps.Clipboard
	previous, _ := clipboard.ReadText()
	if err := writeDictation(clipboard, text, settings.CopyToClipboard); err != nil {
		f.report(err, dictationClipboardMessage(err))
		return
	}
	f.deps.PaletteHidden()
	target := f.target
	f.deps.Background("dictation.paste", func() {
		if err := f.deps.Paster.PasteIntoFrontmost(target); err != nil {
			f.report(err, dictationPasteMessage(err))
			return
		}
		if !settings.CopyToClipboard {
			time.Sleep(f.deps.RestoreDelay)
			restoreDictationClipboard(clipboard, text, previous)
		}
	})
}
