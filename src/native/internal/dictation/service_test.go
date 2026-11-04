package dictation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// scriptedTranscriber records every file it is sent and answers with the scripted texts, in order.
type scriptedTranscriber struct {
	mu      sync.Mutex
	texts   []string
	err     error
	block   chan struct{}
	ids     []string
	files   [][]byte
	prepare []string
}

func (s *scriptedTranscriber) Transcribe(ctx context.Context, operationID string, file []byte) (string, error) {
	s.mu.Lock()
	s.ids = append(s.ids, operationID)
	s.files = append(s.files, file)
	block := s.block
	s.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return "", s.err
	}
	text := s.texts[0]
	s.texts = s.texts[1:]
	return text, nil
}

func (s *scriptedTranscriber) Prepare(_ context.Context, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepare = append(s.prepare, operationID)
	return nil
}

// harness wires a Service to fakes and records the levels the indicator would draw.
type harness struct {
	mic         *fake.Microphone
	codec       *fake.OpusCodec
	transcriber *scriptedTranscriber
	service     *Service
	levels      chan float64
}

func newHarness(t *testing.T) harness {
	t.Helper()
	h := harness{
		mic:         fake.NewMicrophone(),
		codec:       fake.NewOpusCodec(),
		transcriber: &scriptedTranscriber{},
		levels:      make(chan float64, 1024),
	}
	h.service = NewService(Config{
		Microphone:  h.mic,
		Codec:       h.codec,
		Transcriber: h.transcriber,
		OnLevel:     func(level float64) { h.levels <- level },
	})
	return h
}

// speech returns seconds of a constant-level signal, enough to exercise the frame and segment logic.
func speech(seconds float64, level int16) []int16 {
	samples := make([]int16, int(seconds*SampleRate))
	for i := range samples {
		samples[i] = level
	}
	return samples
}

func TestStopTranscribesTheCapturedSpeechAsOneWebMFile(t *testing.T) {
	h := newHarness(t)
	h.transcriber.texts = []string{"  hello world  "}
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	text, err := h.service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("text = %q, want the trimmed transcript", text)
	}
	if len(h.transcriber.files) != 1 || !bytes.HasPrefix(h.transcriber.files[0], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		t.Fatalf("transcriber got %d files; the first must be WebM", len(h.transcriber.files))
	}
	if !strings.HasPrefix(h.transcriber.ids[0], "dictation-") {
		t.Fatalf("operation id = %q, want a dictation- key", h.transcriber.ids[0])
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after Stop = %s, want idle", got)
	}
}

func TestLongDictationsAreSentAsOneMinuteSegmentsInOrder(t *testing.T) {
	h := newHarness(t)
	h.transcriber.texts = []string{"first part", "second part"}
	h.mic.Queue(speech(61, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	text, err := h.service.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if text != "first part second part" {
		t.Fatalf("text = %q, want both segments joined by a space", text)
	}
	if len(h.transcriber.ids) != 2 {
		t.Fatalf("got %d requests, want 2 segments", len(h.transcriber.ids))
	}
	if h.transcriber.ids[1] != h.transcriber.ids[0]+"-1" {
		t.Fatalf("second segment id = %q, want the first id with a -1 suffix", h.transcriber.ids[1])
	}
}

func TestSilentRecordingSendsNothingAndReturnsNoText(t *testing.T) {
	h := newHarness(t)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	text, err := h.service.Stop()
	if err != nil || text != "" {
		t.Fatalf("Stop = %q, %v; want an empty transcript and no error", text, err)
	}
	if len(h.transcriber.ids) != 0 {
		t.Fatalf("a recording with no audio reached the backend: %v", h.transcriber.ids)
	}
}

func TestStartPreparesTheBackendUnderTheSameOperationID(t *testing.T) {
	h := newHarness(t)
	h.transcriber.texts = []string{"ok"}
	h.mic.Queue(speech(0.1, 100)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	h.transcriber.mu.Lock()
	defer h.transcriber.mu.Unlock()
	if len(h.transcriber.prepare) != 1 || h.transcriber.prepare[0] != h.transcriber.ids[0] {
		t.Fatalf("prepare = %v, transcribe = %v; want one preparation under the request's key", h.transcriber.prepare, h.transcriber.ids)
	}
}

func TestStartReportsTheMicrophoneFailureAndStaysIdle(t *testing.T) {
	for _, want := range []error{platform.ErrNotFound, platform.ErrDenied, platform.ErrUnsupported} {
		h := newHarness(t)
		h.mic.FailNext(fake.OpMicrophoneOpen, want)
		if err := h.service.Start("gone"); !errors.Is(err, want) {
			t.Fatalf("Start error = %v, want %v", err, want)
		}
		if got := h.service.Snapshot().Phase; got != PhaseIdle {
			t.Fatalf("phase after a refused microphone = %s, want idle", got)
		}
	}
}

func TestStartWrapsAMissingOpusLibraryAndReleasesTheMicrophone(t *testing.T) {
	h := newHarness(t)
	h.codec.FailNext("opus.new", platform.ErrUnsupported)
	err := h.service.Start("default")
	if !errors.Is(err, ErrCodec) || !errors.Is(err, platform.ErrUnsupported) {
		t.Fatalf("Start error = %v, want ErrCodec wrapping ErrUnsupported", err)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase = %s, want idle", got)
	}
}

func TestStartWhileRunningIsRefusedAndStopWhileIdleToo(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.Stop(); !errors.Is(err, ErrNotRecording) {
		t.Fatalf("Stop while idle = %v, want ErrNotRecording", err)
	}
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.service.Start("default"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Start = %v, want ErrBusy", err)
	}
	_, _ = h.service.Stop()
}

func TestCancelDropsTheRecordingWithoutTranscribing(t *testing.T) {
	h := newHarness(t)
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	h.service.Cancel()
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after Cancel = %s, want idle", got)
	}
	if len(h.transcriber.ids) != 0 {
		t.Fatalf("a cancelled dictation reached the backend")
	}
}

func TestCancelDuringTranscriptionReturnsCancelled(t *testing.T) {
	h := newHarness(t)
	h.transcriber.block = make(chan struct{})
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := h.service.Stop()
		result <- err
	}()
	for h.service.Snapshot().Phase != PhaseTranscribing {
		h.service.Snapshot()
	}
	h.service.Cancel()
	if err := <-result; !errors.Is(err, ErrCancelled) {
		t.Fatalf("Stop after Cancel = %v, want ErrCancelled", err)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase = %s, want idle", got)
	}
}

func TestTranscriptionFailureIsReturnedAndTheServiceIsReusable(t *testing.T) {
	h := newHarness(t)
	failure := errors.New("backend said no")
	h.transcriber.err = failure
	h.mic.Queue(speech(0.5, 9000)...)
	_ = h.service.Start("default")
	if _, err := h.service.Stop(); !errors.Is(err, failure) {
		t.Fatalf("Stop = %v, want the backend failure", err)
	}
	h.transcriber.err = nil
	h.transcriber.texts = []string{"again"}
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start after a failure: %v", err)
	}
}

func TestLevelFollowsTheCapturedSignal(t *testing.T) {
	h := newHarness(t)
	h.transcriber.texts = []string{"loud"}
	h.mic.Queue(speech(0.5, 16384)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var peak float64
	for len(h.levels) > 0 {
		peak = max(peak, <-h.levels)
	}
	if peak < 0.49 || peak > 0.51 {
		t.Fatalf("peak level = %v, want half scale for a half-scale signal", peak)
	}
}

func TestDeviceChoiceReachesTheMicrophone(t *testing.T) {
	h := newHarness(t)
	if err := h.service.Start("usb-mic"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, _ = h.service.Stop()
	if got := h.mic.Opened(); len(got) != 1 || got[0] != "usb-mic" {
		t.Fatalf("opened devices = %v, want usb-mic", got)
	}
}
