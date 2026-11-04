package app

import (
	"testing"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestTheWaitingIndicatorNamesTheDeviceItWaitsFor(t *testing.T) {
	h := newDictationHarness(t)
	h.set.Microphone.SetDevices(platform.MicrophoneDevice{ID: "usb", Title: "USB Mic", Default: true})
	h.flow.Toggle()

	named := false
	for _, state := range h.panel.stateList() {
		if state.Mode == palindicator.ModeWaitingForMicrophone && state.Detail == "Waiting for USB Mic" {
			named = true
		}
	}
	if !named {
		t.Fatalf("states = %+v, want the waiting state to name the device", h.panel.stateList())
	}
}

func TestASilentHeadsetFailsWithTheNotReadyCopy(t *testing.T) {
	h := newDictationHarness(t)
	h.set.Microphone.SetDevices(platform.MicrophoneDevice{ID: "airpods", Title: "AirPods Pro", Default: true})
	h.flow.Toggle()
	got := h.panel.last()
	if got.Mode != palindicator.ModeFailed || got.Detail != dict.FailureMessage(dict.ErrNotReady) {
		t.Fatalf("indicator = %+v, want the not-ready failure", got)
	}
}
