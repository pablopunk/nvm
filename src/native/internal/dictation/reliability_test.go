package dictation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// errTransientENOTSUP is how a passing OS failure reaches the dictation: a raw errno that errors.Is matches to
// platform.ErrUnsupported, which used to read as "not available on this system".
var errTransientENOTSUP = fmt.Errorf("the microphone stream could not start: fork/exec helper: %w", syscall.ENOTSUP)

// brokenMicrophone opens one stream that delivers samples and then fails with fault, and fails every later open with
// fault, so the capture can never recover.
type brokenMicrophone struct {
	*fake.Microphone
	mu      sync.Mutex
	samples []int16
	fault   error
	opens   int
}

func (m *brokenMicrophone) Open(context.Context, string, platform.AudioFormat) (platform.MicrophoneStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opens++
	if m.opens > 1 {
		return nil, m.fault
	}
	return &failingStream{samples: m.samples, fault: m.fault}, nil
}

// failingStream delivers its samples, then fails with fault.
type failingStream struct {
	mu      sync.Mutex
	samples []int16
	fault   error
}

func (s *failingStream) Read(pcm []int16) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.samples) == 0 {
		return 0, s.fault
	}
	n := copy(pcm, s.samples)
	s.samples = s.samples[n:]
	return n, nil
}

func (s *failingStream) Close() error { return nil }

func TestAMidRecordingCaptureErrorKeepsTheAudioInHistoryWithRetry(t *testing.T) {
	rig := newEngineRig(t, true)
	stopped := errors.New("the microphone stopped: error: device removed")
	broken := &brokenMicrophone{Microphone: rig.mic, samples: speech(0.5, 9000), fault: stopped}
	rig.engine.service.cfg.Microphone = broken
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-currentOperation(rig.engine.service).done
	_, err := rig.engine.Stop()
	if err == nil || !strings.HasSuffix(err.Error(), SavedRecordingNotice) {
		t.Fatalf("Stop error = %v, want the failure with the saved-recording notice", err)
	}
	if broken.opens != defaultRetryAttempts {
		t.Fatalf("microphone opened %d times, want the stream reopened until the %d attempts were spent", broken.opens, defaultRetryAttempts)
	}
	saved, _ := rig.engine.Recordings()
	if len(saved) != 1 || saved[0].SegmentCount != 1 || saved[0].Failure == "" {
		t.Fatalf("history recordings = %+v, want the captured audio listed with its failure", saved)
	}
	if len(rig.transcriber.ids()) != 0 {
		t.Fatal("a failed capture was sent before the user asked for it")
	}
	rig.transcriber.texts = []string{"what was said before the failure"}
	if text, err := rig.engine.Retry(context.Background(), saved[0].ID); err != nil || text != "what was said before the failure" {
		t.Fatalf("Retry = %q, %v; want the saved audio transcribed", text, err)
	}
}

func TestAStreamThatFailsMidRecordingIsReopenedAndKeepsEveryWord(t *testing.T) {
	h := newHarness(t)
	h.transcriber.texts = []string{"both halves"}
	flaky := &reopeningMicrophone{Microphone: h.mic, first: &failingStream{samples: speech(0.5, 9000), fault: errTransientENOTSUP}}
	h.service.cfg.Microphone = flaky
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start(""); err != nil {
		t.Fatalf("Start: %v", err)
	}
	op := currentOperation(h.service)
	<-op.done
	if op.frames != 50 {
		t.Fatalf("captured %d frames, want 50: half a second from each stream", op.frames)
	}
	text, err := h.service.Stop()
	if err != nil || text != "both halves" {
		t.Fatalf("Stop = %q, %v; want the transcript of both streams", text, err)
	}
	if got := len(h.mic.Opened()); got != 1 {
		t.Fatalf("reopened %d times, want once", got)
	}
}

// currentOperation returns the dictation the service runs, so a test can wait for its capture to end.
func currentOperation(s *Service) *operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

// reopeningMicrophone hands out first on the first open and the fake's queued stream afterwards.
type reopeningMicrophone struct {
	*fake.Microphone
	mu    sync.Mutex
	first platform.MicrophoneStream
}

func (m *reopeningMicrophone) Open(ctx context.Context, deviceID string, format platform.AudioFormat) (platform.MicrophoneStream, error) {
	m.mu.Lock()
	first := m.first
	m.first = nil
	m.mu.Unlock()
	if first != nil {
		return first, nil
	}
	return m.Microphone.Open(ctx, deviceID, format)
}

func TestATransientStartFailureIsRetriedThenSucceeds(t *testing.T) {
	h := newHarness(t)
	var waits []time.Duration
	var warned []string
	h.service.cfg.Retry.Sleep = func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	}
	h.service.cfg.Warn = func(event string, _ map[string]any) { warned = append(warned, event) }
	h.mic.FailNext(fake.OpMicrophoneOpen, errTransientENOTSUP)
	h.mic.FailNext(fake.OpMicrophoneOpen, platform.ErrNotFound)
	h.mic.FailNext(fake.OpMicrophoneOpen, errors.New("the microphone stopped: error: AVFoundation -11800"))
	h.transcriber.texts = []string{"after three hiccups"}
	h.mic.Queue(speech(0.5, 9000)...)
	if err := h.service.Start(""); err != nil {
		t.Fatalf("Start = %v, want the start retried until the microphone opened", err)
	}
	if text, err := h.service.Stop(); err != nil || text != "after three hiccups" {
		t.Fatalf("Stop = %q, %v", text, err)
	}
	if len(waits) != 3 || waits[0] != time.Second || waits[2] != 4*time.Second {
		t.Fatalf("waits = %v, want three backoffs of 1, 2 and 4 seconds", waits)
	}
	if strings.Count(strings.Join(warned, ","), "dictation.start-retry") != 3 {
		t.Fatalf("warnings = %v, want each retried start logged", warned)
	}
}

func TestAStartThatKeepsFailingTransientlyGivesUpAfterTheAttempts(t *testing.T) {
	h := newHarness(t)
	for range defaultRetryAttempts {
		h.mic.FailNext(fake.OpMicrophoneOpen, errTransientENOTSUP)
	}
	err := h.service.Start("")
	if !errors.Is(err, syscall.ENOTSUP) {
		t.Fatalf("Start = %v, want the last failure once the attempts are spent", err)
	}
	if message := FailureMessage(err); strings.Contains(message, "not available on this system") {
		t.Fatalf("FailureMessage = %q; a passing OS error must not read as an unsupported system", message)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase = %s, want idle", got)
	}
}

func TestATransientTranscriptionFailureRetriesUpToTheCapAndKeepsTheAudio(t *testing.T) {
	rig := newEngineRig(t, true)
	offline := &backend.Error{Kind: backend.KindOffline, Message: "dial tcp", Cause: fmt.Errorf("connect: %w", syscall.EOPNOTSUPP)}
	flaky := &flakyTranscriber{outcomes: []error{offline}}
	rig.engine.service.cfg.Transcriber = flaky
	rig.mic.Queue(speech(0.5, 9000)...)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, err := rig.engine.Stop()
	if len(flaky.calls) != defaultRetryAttempts {
		t.Fatalf("%d attempts, want %d", len(flaky.calls), defaultRetryAttempts)
	}
	want := "Cloud transcription is unavailable. Check your connection and try again. " + SavedRecordingNotice
	if err == nil || err.Error() != want {
		t.Fatalf("Stop error = %v, want %q", err, want)
	}
	saved, _ := rig.engine.Recordings()
	if len(saved) != 1 || !strings.Contains(saved[0].Failure, "Check your connection") {
		t.Fatalf("history recordings = %+v, want the audio kept with the reason", saved)
	}
}

func TestAPermanentFailureFailsAtOnceAndStillKeepsTheAudio(t *testing.T) {
	rig := newEngineRig(t, true)
	flaky := &flakyTranscriber{outcomes: []error{backendFailure(backend.KindUnauthorized)}}
	rig.engine.service.cfg.Transcriber = flaky
	rig.mic.Queue(speech(0.5, 9000)...)
	_ = rig.engine.Start(api.DictationStartOptions{})
	_, err := rig.engine.Stop()
	if len(flaky.calls) != 1 {
		t.Fatalf("%d attempts, want one: a refusal is not retried", len(flaky.calls))
	}
	if err == nil || !strings.HasPrefix(err.Error(), "Sign in to use API dictation.") || !strings.HasSuffix(err.Error(), SavedRecordingNotice) {
		t.Fatalf("Stop error = %v, want the refusal and the saved-recording notice", err)
	}
	if saved, _ := rig.engine.Recordings(); len(saved) != 1 {
		t.Fatalf("history recordings = %+v, want the audio kept", saved)
	}
}

func TestACrashMidTranscriptionLeavesTheRecordingForTheNextLaunch(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.transcriber.block = make(chan struct{})
	rig.transcriber.texts = []string{"answered after the crash"}
	rig.mic.Queue(speech(0.5, 9000)...)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_, _ = rig.engine.Stop()
	}()
	for len(rig.transcriber.ids()) == 0 {
		time.Sleep(time.Millisecond)
	}
	if listed, _ := rig.engine.Recordings(); len(listed) != 0 {
		t.Fatalf("the dictation on its way is listed in the history: %+v", listed)
	}

	nextLaunch := recordings.New(rig.files, "/data/dictation-recordings", time.Now)
	saved, err := nextLaunch.List()
	if err != nil || len(saved) != 1 || saved[0].SegmentCount != 1 || saved[0].Failure != "" {
		t.Fatalf("recordings after the crash = %+v, %v; want the unfinished dictation", saved, err)
	}
	if segments, err := nextLaunch.Load(saved[0].ID); err != nil || len(segments) != 1 || len(segments[0]) == 0 {
		t.Fatalf("Load after the crash = %d segments, %v; want the audio", len(segments), err)
	}
	close(rig.transcriber.block)
	<-stopped
}

func TestACrashMidRecordingLeavesTheAudioSoFarOnDisk(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.Queue(speech(1.5, 9000)...)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	nextLaunch := recordings.New(rig.files, "/data/dictation-recordings", time.Now)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if saved, _ := nextLaunch.List(); len(saved) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing reached the disk while the dictation was recording")
		}
		time.Sleep(time.Millisecond)
	}
	rig.engine.service.Cancel()
}

func TestCancelAfterSpeechKeepsTheRecordingMarkedCancelled(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.Queue(speech(0.5, 9000)...)
	if err := rig.engine.Start(api.DictationStartOptions{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rig.engine.Cancel(); err != nil {
		t.Fatal(err)
	}
	saved, _ := rig.engine.Recordings()
	if len(saved) != 1 || saved[0].Failure != cancelledRecordingNote {
		t.Fatalf("history recordings = %+v, want the cancelled audio kept", saved)
	}
	if len(rig.transcriber.ids()) != 0 {
		t.Fatal("a cancelled dictation reached the backend")
	}
}

func TestADictationWithNoSoundIsNotKept(t *testing.T) {
	rig := newEngineRig(t, true)
	rig.mic.Queue(speech(0.5, 0)...)
	_ = rig.engine.Start(api.DictationStartOptions{})
	_ = rig.engine.Cancel()
	if saved, _ := rig.engine.Recordings(); len(saved) != 0 {
		t.Fatalf("history recordings = %+v, want digital silence dropped", saved)
	}
}

func TestAReadinessTimeoutReopensTheMicrophoneUntilItIsHeard(t *testing.T) {
	h := newHarness(t)
	withBluetoothDefault(h)
	h.service.cfg.ReadyTimeout = 5 * time.Millisecond
	h.service.cfg.Microphone = &reopeningMicrophone{Microphone: h.mic, first: &hangingStream{closed: make(chan struct{})}}
	h.mic.Queue(frames(3, 9000)...)
	h.transcriber.texts = []string{"heard on the second open"}
	if err := h.service.Start(""); err != nil {
		t.Fatalf("Start = %v, want the readiness wait retried on a fresh stream", err)
	}
	if text, err := h.service.Stop(); err != nil || text != "heard on the second open" {
		t.Fatalf("Stop = %q, %v", text, err)
	}
}

func TestFailureClassification(t *testing.T) {
	permanent := []error{
		platform.ErrDenied, platform.ErrUnsupported, fmt.Errorf("%w: %w", ErrCodec, platform.ErrUnsupported), ErrCancelled,
		backendFailure(backend.KindUnauthorized), backendFailure(backend.KindInsufficientCredits),
		backendFailure(backend.KindIncompatible), recordings.ErrNotFound,
	}
	for _, err := range permanent {
		if transientFailure(err) {
			t.Errorf("%v is retried, want it to fail at once", err)
		}
	}
	transient := []error{
		errTransientENOTSUP, platform.ErrNotFound, ErrNotReady, errors.New("unknown"),
		backendFailure(backend.KindOffline), backendFailure(backend.KindTimeout), backendFailure(backend.KindServer),
		backendFailure(backend.KindRateLimited), backendFailure(backend.KindBadResponse),
	}
	for _, err := range transient {
		if !transientFailure(err) {
			t.Errorf("%v fails at once, want it retried", err)
		}
	}
	if got := FailureMessage(platform.ErrUnsupported); got != "Dictation is not available on this system yet." {
		t.Errorf("FailureMessage(ErrUnsupported) = %q", got)
	}
	if got := FailureMessage(errTransientENOTSUP); strings.Contains(got, "not available on this system") {
		t.Errorf("FailureMessage(ENOTSUP) = %q, want a passing failure", got)
	}
}
