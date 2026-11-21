package dictation

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func TestReadyAfterThreeFramesOfAnUnbrokenStreamWithSound(t *testing.T) {
	var tracker readinessTracker
	sound := []bool{false, true, true, true}
	var ready []bool
	for index, hasSound := range sound {
		ready = append(ready, tracker.consume(float64(index)*FrameMillis, hasSound))
	}
	if want := []bool{false, false, false, true}; !slices.Equal(ready, want) {
		t.Fatalf("ready = %v, want %v", ready, want)
	}
}

func TestSilenceForFiveSecondsIsReadyAnyway(t *testing.T) {
	var tracker readinessTracker
	ready := false
	for ms := 0.0; ms <= readinessSilentMS; ms += FrameMillis {
		ready = tracker.consume(ms, false)
	}
	if !ready {
		t.Fatalf("five seconds of silence did not count as ready")
	}
}

func TestAGapInTheStreamStartsTheCountAgain(t *testing.T) {
	var tracker readinessTracker
	tracker.consume(0, true)
	tracker.consume(20, true)
	if tracker.consume(1000, true) {
		t.Fatalf("a frame after a gap counted towards the earlier streak")
	}
}

func TestBluetoothNamesNeedTheWait(t *testing.T) {
	for name, want := range map[string]bool{
		"AirPods Pro":            true,
		"Bose QC35":              true,
		"Jabra Evolve":           true,
		"Bluetooth Headset":      true,
		"MacBook Pro Microphone": false,
		"USB Mic":                false,
	} {
		if got := NeedsBluetoothReadiness(name); got != want {
			t.Errorf("NeedsBluetoothReadiness(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDeviceTitleFindsTheChosenOrTheDefaultDevice(t *testing.T) {
	devices := []platform.MicrophoneDevice{
		{ID: "usb", Title: "USB Mic"},
		{ID: "airpods", Title: "AirPods Pro", Default: true},
	}
	if got := DeviceTitle(devices, "usb"); got != "USB Mic" {
		t.Fatalf("chosen device title = %q", got)
	}
	if got := DeviceTitle(devices, ""); got != "AirPods Pro" {
		t.Fatalf("default device title = %q", got)
	}
	if got := DeviceTitle(devices, "gone"); got != "" {
		t.Fatalf("missing device title = %q, want empty", got)
	}
}
