package dictation

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestAFailedChosenDeviceFallsBackToTheDefaultAndWarns(t *testing.T) {
	h := newHarness(t)
	var warned []string
	h.service.cfg.Warn = func(event string, _ map[string]any) { warned = append(warned, event) }
	h.mic.FailNext(fake.OpMicrophoneOpen, platform.ErrNotFound)
	if err := h.service.Start("usb-mic"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, _ = h.service.Stop()
	if got := h.mic.Opened(); len(got) != 1 || got[0] != "" {
		t.Fatalf("opened devices = %v, want the default device", got)
	}
	if len(warned) != 1 || warned[0] != "dictation.device-fallback" {
		t.Fatalf("warnings = %v, want one device fallback", warned)
	}
}

func TestAFailedFallbackReportsTheChosenDevicesError(t *testing.T) {
	h := newHarness(t)
	h.mic.FailNext(fake.OpMicrophoneOpen, platform.ErrNotFound)
	h.mic.FailNext(fake.OpMicrophoneOpen, platform.ErrUnsupported)
	if err := h.service.Start("usb-mic"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Start error = %v, want the chosen device's ErrNotFound", err)
	}
}

func TestARefusedMicrophoneIsNotRetriedOnTheDefault(t *testing.T) {
	h := newHarness(t)
	h.mic.FailNext(fake.OpMicrophoneOpen, platform.ErrDenied)
	if err := h.service.Start("usb-mic"); !errors.Is(err, platform.ErrDenied) {
		t.Fatalf("Start error = %v, want ErrDenied", err)
	}
	if got := h.mic.Opened(); len(got) != 0 {
		t.Fatalf("opened devices = %v, want none after a refusal", got)
	}
}

// gatedMicrophone holds Open until the test releases it, so a Cancel can land while the microphone is opening.
type gatedMicrophone struct {
	*fake.Microphone
	opening chan struct{}
	gate    chan struct{}
}

func (m gatedMicrophone) Open(ctx context.Context, deviceID string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	m.opening <- struct{}{}
	<-m.gate
	return m.Microphone.Open(ctx, deviceID, format)
}

func TestCancelWhileTheMicrophoneOpensEndsTheStartAndClosesTheLateStream(t *testing.T) {
	h := newHarness(t)
	gate := gatedMicrophone{Microphone: h.mic, opening: make(chan struct{}, 1), gate: make(chan struct{})}
	h.service.cfg.Microphone = gate
	started := make(chan error, 1)
	go func() { started <- h.service.Start("default") }()
	<-gate.opening
	if got := h.service.Snapshot().Phase; got != PhaseRecording {
		t.Fatalf("phase while opening = %s, want recording", got)
	}
	h.service.Cancel()
	close(gate.gate)
	if err := <-started; !errors.Is(err, ErrCancelled) {
		t.Fatalf("Start after Cancel = %v, want ErrCancelled", err)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after a cancelled start = %s, want idle", got)
	}
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("a Start after the cancel: %v", err)
	}
	_, _ = h.service.Stop()
}
