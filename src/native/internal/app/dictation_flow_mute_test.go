package app

import (
	"testing"

	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestTheOutputIsMutedWhileRecordingAndGivenBackWhenItStops(t *testing.T) {
	h := newDictationHarness(t)
	h.speak("hello")
	h.flow.Toggle()
	if muted, _ := h.set.OutputMute.Muted(); !muted {
		t.Fatalf("output not muted while recording")
	}
	h.flow.Toggle()
	if muted, _ := h.set.OutputMute.Muted(); muted {
		t.Fatalf("output still muted after the dictation stopped")
	}
}

func TestCancelGivesTheOutputBackItsState(t *testing.T) {
	h := newDictationHarness(t)
	h.set.OutputMute.SetState(false)
	h.flow.Toggle()
	h.flow.Press(palindicator.ControlCancel)
	if changes := h.set.OutputMute.Changes(); len(changes) != 2 || changes[1] {
		t.Fatalf("output changes = %v, want mute then restore", changes)
	}
}

func TestAFailedStartGivesTheOutputBack(t *testing.T) {
	h := newDictationHarness(t)
	h.set.Microphone.FailNext(fake.OpMicrophoneOpen, platform.ErrDenied)
	h.flow.Toggle()
	if muted, _ := h.set.OutputMute.Muted(); muted {
		t.Fatalf("output left muted after the microphone refused")
	}
}

func TestAnOutputThatCannotBeMutedDoesNotStopTheDictation(t *testing.T) {
	h := newDictationHarness(t)
	h.set.OutputMute.FailNext("output.muted", platform.ErrUnsupported)
	h.speak("still works")
	h.flow.Toggle()
	h.flow.Toggle()
	if len(h.pastes) != 1 {
		t.Fatalf("pastes = %+v, want the dictation to go on unmuted", h.pastes)
	}
}
