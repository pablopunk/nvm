package app

import (
	"testing"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestThePasteGoesToTheAppThatHadFocusWhenDictationStarted(t *testing.T) {
	h := newDictationHarness(t)
	h.frontmost = "com.apple.Notes"
	h.speak("into notes")
	h.flow.Toggle()
	h.frontmost = "com.apple.Safari"
	h.flow.Toggle()

	if len(h.pastes) != 1 || h.pastes[0].target != "com.apple.Notes" {
		t.Fatalf("pastes = %+v, want one paste aimed at the app that started the dictation", h.pastes)
	}
}

func TestAPasteIntoAnotherAppToastsLogsAndKeepsTheTextOnTheClipboard(t *testing.T) {
	h := newDictationHarness(t)
	h.pasteErr = platform.ErrConflict
	h.speak("kept")
	h.flow.Toggle()
	h.flow.Toggle()

	if want := dictationPasteMessage(platform.ErrConflict); len(h.toasts) != 1 || h.toasts[0] != want {
		t.Fatalf("toasts = %v, want %q", h.toasts, want)
	}
	if !containsString(h.warned, "dictation.failed") {
		t.Fatalf("warnings = %v, want the failed paste logged", h.warned)
	}
	if text, _ := h.set.Clipboard.ReadText(); text != "kept" {
		t.Fatalf("clipboard = %q, want the text kept for a manual paste", text)
	}
}

func TestAnUnknownTargetPastesIntoWhateverAppIsInFront(t *testing.T) {
	h := newDictationHarness(t)
	h.frontmostErr = platform.ErrUnsupported
	h.speak("anywhere")
	h.flow.Toggle()
	h.flow.Toggle()

	if len(h.pastes) != 1 || h.pastes[0].target != "" {
		t.Fatalf("pastes = %+v, want one paste with no target", h.pastes)
	}
	if containsString(h.warned, "dictation.target-unknown") {
		t.Fatalf("an unsupported target is not a problem to log, got %v", h.warned)
	}
}

func TestAMicrophoneRefusalExplainsItWithASettingsButtonAndAToastAndALogEntry(t *testing.T) {
	h := newDictationHarness(t)
	h.set.Microphone.FailNext(fake.OpMicrophoneOpen, platform.ErrDenied)
	h.flow.Toggle()

	got := h.panel.last()
	if got.Mode != palindicator.ModeFailed || got.Control != palindicator.ControlMicrophoneSettings {
		t.Fatalf("indicator = %+v, want a failure with the microphone settings button", got)
	}
	if got.Detail != dict.FailureMessage(platform.ErrDenied) {
		t.Fatalf("detail = %q, want the refusal copy", got.Detail)
	}
	if len(h.toasts) != 1 || h.toasts[0] != got.Detail {
		t.Fatalf("toasts = %v, want the refusal shown as a toast too", h.toasts)
	}
	if !containsString(h.warned, "dictation.failed") {
		t.Fatalf("warnings = %v, want the refusal logged", h.warned)
	}
}

func TestPressingOpenMicrophoneSettingsOpensThePaneAndClosesTheIndicator(t *testing.T) {
	h := newDictationHarness(t)
	h.set.Microphone.FailNext(fake.OpMicrophoneOpen, platform.ErrDenied)
	h.flow.Toggle()
	h.flow.Press(palindicator.ControlMicrophoneSettings)

	if h.settingsOpen != 1 {
		t.Fatalf("settings opened %d times, want once", h.settingsOpen)
	}
	if got := lastOf(h.panel.eventList()); got != "hide" {
		t.Fatalf("last indicator event = %q, want hide", got)
	}
}

func TestAFailedSettingsPaneIsToastedAndLogged(t *testing.T) {
	h := newDictationHarness(t)
	h.settingsErr = platform.ErrUnsupported
	h.flow.Press(palindicator.ControlMicrophoneSettings)

	if len(h.toasts) != 1 || h.toasts[0] != dictationSettingsFailureMessage {
		t.Fatalf("toasts = %v, want the settings failure", h.toasts)
	}
	if !containsString(h.warned, "dictation.failed") {
		t.Fatalf("warnings = %v, want the failure logged", h.warned)
	}
}

func TestTheListeningIndicatorOffersCancelAndPressingItStopsTheDictation(t *testing.T) {
	h := newDictationHarness(t)
	h.speak("never")
	h.flow.Toggle()
	if got := h.panel.last(); got.Mode != palindicator.ModeListening || got.Control != palindicator.ControlCancel {
		t.Fatalf("indicator = %+v, want listening with cancel", got)
	}
	h.flow.Press(palindicator.ControlCancel)
	if len(h.pastes) != 0 || len(h.transcriber.segments) != 0 {
		t.Fatalf("a cancelled dictation was sent or pasted")
	}
	if got := lastOf(h.panel.eventList()); got != "hide" {
		t.Fatalf("last indicator event = %q, want hide", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
