package dictation

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// scriptedTranscriber records every segment it is sent and every preparation release, and answers with the scripted
// texts in order.
type scriptedTranscriber struct {
	mu         sync.Mutex
	texts      []string
	err        error
	block      chan struct{}
	segments   []Segment
	prepare    []string
	prepareErr error
	released   []bool
	releaseID  []string
}

func (s *scriptedTranscriber) Transcribe(ctx context.Context, segment Segment) (string, error) {
	s.mu.Lock()
	s.segments = append(s.segments, segment)
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
	return s.prepareErr
}

func (s *scriptedTranscriber) CancelPreparation(_ context.Context, operationID string, userCancelled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseID = append(s.releaseID, operationID)
	s.released = append(s.released, userCancelled)
	return nil
}

func (s *scriptedTranscriber) ids() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, segment := range s.segments {
		out = append(out, segment.OperationID)
	}
	return out
}

func (s *scriptedTranscriber) files() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out [][]byte
	for _, segment := range s.segments {
		out = append(out, segment.File)
	}
	return out
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
		OnLevel:     func(level float64, _ func() bool) { h.levels <- level },
		Background:  func(_ string, fn func()) { fn() },
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
	files := h.transcriber.files()
	if len(files) != 1 || !bytes.HasPrefix(files[0], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		t.Fatalf("transcriber got %d files; the first must be WebM", len(files))
	}
	if ids := h.transcriber.ids(); !strings.HasPrefix(ids[0], "dictation-") {
		t.Fatalf("operation id = %q, want a dictation- key", ids[0])
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
	ids := h.transcriber.ids()
	if len(ids) != 2 {
		t.Fatalf("got %d requests, want 2 segments", len(ids))
	}
	if ids[1] != ids[0]+"-1" {
		t.Fatalf("second segment id = %q, want the first id with a -1 suffix", ids[1])
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
	if ids := h.transcriber.ids(); len(ids) != 0 {
		t.Fatalf("a recording with no audio reached the backend: %v", ids)
	}
}

func TestStartPreparesTheBackendUnderTheSameOperationIDAndSendsItWithTheFirstSegment(t *testing.T) {
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
	segments := h.transcriber.segments
	if len(h.transcriber.prepare) != 1 || len(segments) != 1 || h.transcriber.prepare[0] != segments[0].OperationID {
		t.Fatalf("prepare = %v, segments = %+v; want one preparation under the request's key", h.transcriber.prepare, segments)
	}
	if !segments[0].Prepared || segments[0].CancellationID != segments[0].OperationID {
		t.Fatalf("segment = %+v, want Prepared with the dictation as its cancellation id", segments[0])
	}
}

func TestAFailedPreparationIsNotSentAsPrepared(t *testing.T) {
	h := newHarness(t)
	h.transcriber.prepareErr = errors.New("admission refused")
	h.transcriber.texts = []string{"ok"}
	h.mic.Queue(speech(0.1, 100)...)
	_ = h.service.Start("default")
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	h.transcriber.mu.Lock()
	defer h.transcriber.mu.Unlock()
	if h.transcriber.segments[0].Prepared {
		t.Fatalf("a refused preparation was sent as prepared")
	}
}

func TestStartReportsTheMicrophoneFailureAndStaysIdle(t *testing.T) {
	for _, want := range []error{platform.ErrNotFound, platform.ErrDenied, platform.ErrUnsupported} {
		h := newHarness(t)
		h.mic.FailNext(fake.OpMicrophoneOpen, want)
		if err := h.service.Start(""); !errors.Is(err, want) {
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
	if ids := h.transcriber.ids(); len(ids) != 0 {
		t.Fatalf("a cancelled dictation reached the backend: %v", ids)
	}
	h.transcriber.mu.Lock()
	defer h.transcriber.mu.Unlock()
	if len(h.transcriber.released) != 1 || !h.transcriber.released[0] {
		t.Fatalf("releases = %v, want one user-cancelled release of the preparation", h.transcriber.released)
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
	// A stopped operation must not publish live levels. Observe capture before stopping it.
	var peak float64
	select {
	case peak = <-h.levels:
	case <-time.After(time.Second):
		t.Fatal("recording did not publish a live microphone level")
	}
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if peak < 0.70 || peak > 0.71 {
		t.Fatalf("peak level = %v, want square-root display scale for a half-scale signal", peak)
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
