package app

import (
	"errors"
	"testing"

	dict "github.com/pablopunk/nvm/native/internal/dictation"
)

// memoryRecordings keeps saved dictations in memory for the recovery tests.
type memoryRecordings struct {
	saved map[string][][]byte
}

func (r memoryRecordings) Save(id string, segments [][]byte) error {
	r.saved[id] = segments
	return nil
}

func (r memoryRecordings) Remove(id string) error {
	delete(r.saved, id)
	return nil
}

func (r memoryRecordings) Load(id string) ([][]byte, error) {
	segments, ok := r.saved[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return segments, nil
}

var _ dict.Recorder = memoryRecordings{}

func TestRecoverTranscribesTheSavedDictationAndReportsItsText(t *testing.T) {
	h := newDictationHarness(t)
	h.recordings.saved["dictation-saved"] = [][]byte{[]byte("audio")}
	h.transcriber.texts = []string{"recovered words"}

	var gotText string
	var gotErr error
	h.flow.Recover("dictation-saved", func(text string, err error) { gotText, gotErr = text, err })
	if gotErr != nil || gotText != "recovered words" {
		t.Fatalf("Recover = %q, %v; want the recovered text", gotText, gotErr)
	}
	if _, ok := h.recordings.saved["dictation-saved"]; !ok {
		t.Fatalf("recovery removed the saved dictation before the caller confirmed it")
	}
}

func TestRecoverWhileSignedOutAsksForSignIn(t *testing.T) {
	h := newDictationHarness(t)
	h.account.signedIn = false
	var gotErr error
	h.flow.Recover("dictation-saved", func(_ string, err error) { gotErr = err })
	if gotErr == nil || gotErr.Error() != dictationSignInMessage {
		t.Fatalf("Recover signed out = %v, want the sign-in message", gotErr)
	}
}
