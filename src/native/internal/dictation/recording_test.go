package dictation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

// fakeRecorder keeps saved segments in memory and records what was removed.
type fakeRecorder struct {
	mu      sync.Mutex
	saved   map[string][][]byte
	removed []string
	saveErr error
	failed  map[string]string
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{saved: map[string][][]byte{}, failed: map[string]string{}}
}

func (r *fakeRecorder) Save(id string, segments [][]byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved[id] = segments
	return nil
}

func (r *fakeRecorder) MarkFailed(id, failure string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed[id] = failure
	return nil
}

func (r *fakeRecorder) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, id)
	delete(r.saved, id)
	return nil
}

func (r *fakeRecorder) Load(id string) ([][]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	segments, ok := r.saved[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return segments, nil
}

func (r *fakeRecorder) removedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.removed)
}

func (r *fakeRecorder) savedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for id := range r.saved {
		ids = append(ids, id)
	}
	return ids
}

func TestAFinishedDictationKeepsItsAudioOnlyUntilTheTextIsIn(t *testing.T) {
	h := newHarness(t)
	recorder := newFakeRecorder()
	h.service.cfg.Recorder = recorder
	h.transcriber.texts = []string{"done"}
	h.mic.Queue(speech(0.5, 9000)...)
	_ = h.service.Start("default")
	if _, err := h.service.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if ids := recorder.savedIDs(); len(ids) != 0 {
		t.Fatalf("saved recordings after success = %v, want none left", ids)
	}
	if removed := recorder.removedIDs(); len(removed) != 1 {
		t.Fatalf("removed = %v, want the audio dropped once the text is in", removed)
	}
}

func TestAFailedTranscriptionKeepsTheRecordingAndSaysSo(t *testing.T) {
	h := newHarness(t)
	recorder := newFakeRecorder()
	h.service.cfg.Recorder = recorder
	h.transcriber.err = errors.New("cloud transcription is unavailable")
	h.mic.Queue(speech(0.5, 9000)...)
	_ = h.service.Start("default")
	_, err := h.service.Stop()

	var saved *SavedError
	if !errors.As(err, &saved) {
		t.Fatalf("Stop error = %v, want a SavedError", err)
	}
	if removed := recorder.removedIDs(); len(removed) != 0 {
		t.Fatalf("a failed dictation removed its recording: %v", removed)
	}
	if message := FailureMessage(err); !strings.HasSuffix(message, SavedRecordingNotice) {
		t.Fatalf("FailureMessage = %q, want the saved-recording notice at the end", message)
	}
}

func TestAFailedSaveDoesNotStopTheDictation(t *testing.T) {
	h := newHarness(t)
	recorder := newFakeRecorder()
	recorder.saveErr = errors.New("disk full")
	var warned []string
	h.service.cfg.Recorder = recorder
	h.service.cfg.Warn = func(event string, _ map[string]any) { warned = append(warned, event) }
	h.transcriber.texts = []string{"still pasted"}
	h.mic.Queue(speech(0.5, 9000)...)
	_ = h.service.Start("default")
	text, err := h.service.Stop()
	if err != nil || text != "still pasted" {
		t.Fatalf("Stop = %q, %v; want the text despite the failed save", text, err)
	}
	if len(warned) != 1 || warned[0] != "dictation.recording-save-failed" {
		t.Fatalf("warnings = %v, want the failed save logged", warned)
	}
}

func TestRetryTranscribesTheSavedSegmentsWithFreshIDsAndKeepsTheRecording(t *testing.T) {
	h := newHarness(t)
	recorder := newFakeRecorder()
	recorder.saved["dictation-saved"] = [][]byte{[]byte("one"), []byte("two")}
	h.service.cfg.Recorder = recorder
	h.transcriber.texts = []string{"first", "second"}

	text, err := h.service.Retry(context.Background(), "dictation-saved")
	if err != nil || text != "first second" {
		t.Fatalf("Retry = %q, %v; want the joined text", text, err)
	}
	ids := h.transcriber.ids()
	if len(ids) != 2 || !strings.HasPrefix(ids[0], "dictation-retry-") || ids[1] != ids[0]+"-1" {
		t.Fatalf("operation ids = %v, want fresh retry ids per segment", ids)
	}
	if removed := recorder.removedIDs(); len(removed) != 0 {
		t.Fatalf("a retry removed the recording: %v", removed)
	}
	if got := h.service.Snapshot().Phase; got != PhaseIdle {
		t.Fatalf("phase after a retry = %s, want idle", got)
	}
}

func TestRetryIsRefusedWhileADictationRuns(t *testing.T) {
	h := newHarness(t)
	recorder := newFakeRecorder()
	recorder.saved["dictation-saved"] = [][]byte{[]byte("one")}
	h.service.cfg.Recorder = recorder
	h.mic.Queue(speech(0.1, 9000)...)
	if err := h.service.Start("default"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := h.service.Retry(context.Background(), "dictation-saved"); !errors.Is(err, ErrBusy) {
		t.Fatalf("Retry during a dictation = %v, want ErrBusy", err)
	}
	h.service.Cancel()
}

func TestRetryWithoutARecorderSaysSo(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.Retry(context.Background(), "dictation-x"); !errors.Is(err, ErrNoRecordings) {
		t.Fatalf("Retry without a recorder = %v, want ErrNoRecordings", err)
	}
}
