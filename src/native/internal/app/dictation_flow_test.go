package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	dict "github.com/pablopunk/nvm/native/internal/dictation"
	"github.com/pablopunk/nvm/native/internal/dictation/cleanup"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// dictationHarness runs a dictationFlow over fakes. Dispatch, Background and the restore delay are inline, so each
// call returns with its whole effect applied; timers are collected and fired by the test.
type dictationHarness struct {
	// ui serializes Dispatch, as the palette's UI thread does, because the capture goroutine reports levels.
	ui          sync.Mutex
	set         *fake.Set
	account     *fakeDictationAccount
	panel       *fakeDictationPanel
	transcriber *fakeDictationTranscriber
	flow        *dictationFlow
	settings    dictationSettings
	hidden      int
	pastes      []pasteRecord
	pasteErr    error
	toasts      []string
	timers      []func()
	frontmost   string
	// frontmostErr is what FrontmostAppID returns; the zero value means the app is known.
	frontmostErr error
	warned       []string
	settingsOpen int
	settingsErr  error
	// completer answers the AI cleanup prompt; nil means the model is not reachable.
	completer cleanup.Completer
	// screenText reads the screen; nil means the screen cannot be read. screenReads counts the readings.
	screenText  func(ctx context.Context) (string, error)
	screenReads int
}

func newDictationHarness(t *testing.T) *dictationHarness {
	t.Helper()
	h := &dictationHarness{
		set:         fake.NewSet(),
		panel:       &fakeDictationPanel{},
		transcriber: &fakeDictationTranscriber{},
	}
	h.set.Clipboard.WriteText("before")
	h.account = &fakeDictationAccount{signedIn: true, transcriber: h.transcriber}
	h.flow = newDictationFlow(dictationDeps{
		Microphone:    h.set.Microphone,
		Codec:         h.set.Opus,
		Clipboard:     h.set.Clipboard,
		Paster:        orderedPaster{harness: h},
		Account:       h.account,
		Settings:      func() dictationSettings { return h.settings },
		Panel:         h.panel,
		Dispatch:      h.dispatch,
		Background:    func(_ string, fn func()) { fn() },
		After:         func(_ time.Duration, fn func()) { h.timers = append(h.timers, fn) },
		PaletteHidden: func() { h.hidden++ },
		Toast:         func(msg string) { h.toasts = append(h.toasts, msg) },
		Warn:          func(event string, _ map[string]any) { h.warned = append(h.warned, event) },
		OpenMicrophoneSettings: func() error {
			h.settingsOpen++
			return h.settingsErr
		},
		Complete: func(ctx context.Context, system, user string) (string, error) {
			if h.completer == nil {
				return "", errors.New("no model")
			}
			return h.completer(ctx, system, user)
		},
		ScreenText: func(ctx context.Context) (string, error) {
			h.screenReads++
			if h.screenText == nil {
				return "", errors.New("no screen")
			}
			return h.screenText(ctx)
		},
		OnDevices: func() {},
	})
	return h
}

func (h *dictationHarness) dispatch(fn func()) {
	h.ui.Lock()
	defer h.ui.Unlock()
	fn()
}

// dictationSpeech is seconds of a steady signal at level.
func dictationSpeech(seconds float64, level int16) []int16 {
	samples := make([]int16, int(seconds*dict.SampleRate))
	for i := range samples {
		samples[i] = level
	}
	return samples
}

func (h *dictationHarness) speak(text string) {
	h.transcriber.texts = append(h.transcriber.texts, text)
	h.set.Microphone.Queue(dictationSpeech(0.5, 9000)...)
}

func TestDictationPastesConcealedTextAfterThePaletteHidesAndRestoresTheClipboard(t *testing.T) {
	h := newDictationHarness(t)
	h.speak("hello world")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.transcriber.segments) != 1 || !h.transcriber.segments[0].Prepared {
		t.Fatalf("segments = %+v, want one prepared segment", h.transcriber.segments)
	}
	if len(h.pastes) != 1 {
		t.Fatalf("pastes = %d, want one", len(h.pastes))
	}
	paste := h.pastes[0]
	if paste.hidden == 0 {
		t.Fatalf("the paste ran before the palette hid")
	}
	if !paste.concealed || paste.text != "hello world" {
		t.Fatalf("at paste the clipboard was %+v, want the transcript marked concealed", paste)
	}
	if text, _ := h.set.Clipboard.ReadText(); text != "before" {
		t.Fatalf("clipboard after the paste = %q, want the previous text restored", text)
	}
	want := []string{"show:waiting", "show:listening", "show:transcribing", "hide"}
	if !equalStrings(h.panel.eventList(), want) {
		t.Fatalf("indicator events = %v, want %v", h.panel.eventList(), want)
	}
}

func TestCopyToClipboardKeepsThePlainTextAndSkipsTheRestore(t *testing.T) {
	h := newDictationHarness(t)
	h.settings.CopyToClipboard = true
	h.speak("kept")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.pastes) != 1 || h.pastes[0].concealed {
		t.Fatalf("pastes = %+v, want one plain paste", h.pastes)
	}
	if text, _ := h.set.Clipboard.ReadText(); text != "kept" {
		t.Fatalf("clipboard = %q, want the dictated text kept", text)
	}
}

func TestCancelWhileRecordingReleasesThePreparationAndClosesTheIndicator(t *testing.T) {
	h := newDictationHarness(t)
	h.speak("never sent")
	h.flow.Toggle()
	h.flow.Cancel()

	if len(h.transcriber.segments) != 0 || len(h.pastes) != 0 {
		t.Fatalf("a cancelled dictation was sent or pasted")
	}
	if len(h.transcriber.released) != 1 || !h.transcriber.released[0] {
		t.Fatalf("released = %v, want one user-cancelled release", h.transcriber.released)
	}
	if got := lastOf(h.panel.eventList()); got != "hide" {
		t.Fatalf("last indicator event = %q, want hide", got)
	}
	h.flow.Toggle()
	if got := h.panel.last(); got.Mode != palindicator.ModeListening {
		t.Fatalf("after a cancel the next Toggle should start listening, got %+v", got)
	}
}

func TestSignedOutExplainsTheRefusalAndNeverOpensTheMicrophone(t *testing.T) {
	h := newDictationHarness(t)
	h.account.signedIn = false
	h.flow.Toggle()

	if got := h.panel.last(); got.Mode != palindicator.ModeFailed || got.Detail != dictationSignInMessage {
		t.Fatalf("indicator = %+v, want the sign-in message", got)
	}
	if opened := h.set.Microphone.Opened(); len(opened) != 0 {
		t.Fatalf("the microphone opened for a signed-out account: %v", opened)
	}
}

func TestBackendSpendCapRefusalNotesTheAccountAndExplainsItInTheIndicator(t *testing.T) {
	h := newDictationHarness(t)
	refusal := &backend.Error{Kind: backend.KindServer, Status: 503, Code: "free_spend_cap_reached", Message: "paused"}
	h.transcriber.err = refusal
	h.speak("refused")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.account.notes) != 1 || !errors.Is(h.account.notes[0], refusal) {
		t.Fatalf("account notes = %v, want the refusal", h.account.notes)
	}
	got := h.panel.last()
	if got.Mode != palindicator.ModeFailed || !strings.Contains(got.Detail, "temporarily paused") {
		t.Fatalf("indicator = %+v, want the free-pool explanation", got)
	}
	if len(h.pastes) != 0 {
		t.Fatalf("a refused dictation was pasted")
	}
	if len(h.timers) != 1 {
		t.Fatalf("timers = %d, want the failure dismiss", len(h.timers))
	}
	h.timers[0]()
	if got := lastOf(h.panel.eventList()); got != "hide" {
		t.Fatalf("after the dismiss the last event = %q, want hide", got)
	}
}

func TestSilentDictationShowsNoSpeechAndPastesNothing(t *testing.T) {
	h := newDictationHarness(t)
	h.flow.Toggle()
	h.flow.Toggle()

	if got := h.panel.last(); got.Mode != palindicator.ModeNoSpeech {
		t.Fatalf("indicator = %+v, want no speech", got)
	}
	if len(h.pastes) != 0 || len(h.transcriber.segments) != 0 {
		t.Fatalf("silence reached the backend or the paste")
	}
}

func TestTheChosenMicrophoneIsOpened(t *testing.T) {
	h := newDictationHarness(t)
	h.settings.DeviceID = "usb-mic"
	h.flow.Toggle()
	h.flow.Cancel()
	if opened := h.set.Microphone.Opened(); len(opened) != 1 || opened[0] != "usb-mic" {
		t.Fatalf("opened = %v, want the chosen device", opened)
	}
}

func TestTheListeningIndicatorShowsTheMeterAndALateLevelNeverReopensIt(t *testing.T) {
	h := newDictationHarness(t)
	h.flow.Toggle()
	h.flow.onLevel(0.5)
	if got := h.panel.last(); got.Mode != palindicator.ModeListening || got.Level != 0.5 {
		t.Fatalf("indicator = %+v, want listening at level 0.5", got)
	}
	h.flow.Cancel()
	shown := len(h.panel.stateList())
	h.flow.onLevel(0.9)
	if len(h.panel.stateList()) != shown {
		t.Fatalf("a level after cancel changed the indicator: %+v", h.panel.stateList()[shown:])
	}
}

func TestAPasteThatTheOSRefusesLeavesTheTextForAManualPaste(t *testing.T) {
	h := newDictationHarness(t)
	h.pasteErr = platform.ErrUnsupported
	h.speak("manual")
	h.flow.Toggle()
	h.flow.Toggle()

	if want := dictationPasteMessage(platform.ErrUnsupported); len(h.toasts) != 1 || h.toasts[0] != want {
		t.Fatalf("toasts = %v, want %q", h.toasts, want)
	}
	if text, _ := h.set.Clipboard.ReadText(); text != "manual" {
		t.Fatalf("clipboard = %q, want the text left for a manual paste", text)
	}
}

func lastOf(events []string) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1]
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
