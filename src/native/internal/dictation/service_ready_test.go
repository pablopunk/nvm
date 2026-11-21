package dictation

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// withBluetoothDefault makes the scripted microphone's default device an AirPods headset.
func withBluetoothDefault(h harness) {
	h.mic.SetDevices(platform.MicrophoneDevice{ID: "airpods", Title: "AirPods Pro", Default: true})
}

// frames returns n frames of sound, exactly n*FrameMillis of audio.
func frames(n int, level int16) []int16 {
	samples := make([]int16, n*samplesPerFrame)
	for index := range samples {
		samples[index] = level
	}
	return samples
}

func TestABluetoothMicrophoneStartsOnceItHearsSound(t *testing.T) {
	h := newHarness(t)
	withBluetoothDefault(h)
	h.transcriber.texts = []string{"first words"}
	h.mic.Queue(frames(3, 9000)...)
	if err := h.service.Start(""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	text, err := h.service.Stop()
	if err != nil || text != "first words" {
		t.Fatalf("Stop = %q, %v; want the transcript of the heard audio", text, err)
	}
}

func TestASilentBluetoothMicrophoneFailsWithNotReady(t *testing.T) {
	h := newHarness(t)
	withBluetoothDefault(h)
	h.mic.Queue(frames(2, 0)...)
	if err := h.service.Start(""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Start = %v, want ErrNotReady", err)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after a silent start = %s, want idle", got)
	}
}

func TestAMicrophoneThatNeverDeliversTimesOut(t *testing.T) {
	h := newHarness(t)
	withBluetoothDefault(h)
	h.service.cfg.Microphone = hangingMicrophone{Microphone: h.mic}
	h.service.cfg.ReadyTimeout = 5 * time.Millisecond
	if err := h.service.Start(""); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Start = %v, want ErrNotReady after the timeout", err)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after a timeout = %s, want idle", got)
	}
}

// hangingMicrophone opens streams whose reads wait until the stream is closed.
type hangingMicrophone struct {
	*fake.Microphone
}

func (m hangingMicrophone) Open(context.Context, string, platform.AudioFormat) (platform.MicrophoneStream, error) {
	return &hangingStream{closed: make(chan struct{})}, nil
}

type hangingStream struct {
	closed chan struct{}
	once   sync.Once
}

func (s *hangingStream) Read([]int16) (int, error) {
	<-s.closed
	return 0, io.EOF
}

func (s *hangingStream) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}
